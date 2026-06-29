package kube

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	pendingPodAlertAfter          = 10 * time.Minute
	gpuPendingPodAlertAfter       = 5 * time.Minute
	gpuUnreadyPodAlertAfter       = 10 * time.Minute
	underRequestedPodAfter        = 15 * time.Minute
	highRestartThreshold    int32 = 3

	gpuAllocationWarningRatio  = 0.80
	gpuAllocationCriticalRatio = 0.95
	underRequestCPUPercent     = 20
	underRequestMemoryPercent  = 20
	nodeUnderRequestPercent    = 50
	minUnderRequestCPUMilli    = 100
	minUnderRequestMemoryBytes = 128 * 1024 * 1024

	unsupportedInsightCategory = "__unsupported__"
)

func (c *Client) BuildInsights(ctx context.Context, namespaces []string, categories ...string) ([]InsightAlert, error) {
	nsList := uniqueStrings(namespaces)
	if len(nsList) == 0 {
		return nil, nil
	}
	insightCategory := normalizeInsightCategory(firstString(categories))
	includeAll := insightCategory == ""
	includeNodes := includeAll || insightCategory == "nodes"
	includeWorkloads := includeAll || insightCategory == "workloads"
	includeGPUChecks := includeAll || insightCategory == "gpu"
	includeNetworking := includeAll || insightCategory == "networking"
	includeStorage := includeAll || insightCategory == "storage"
	includeSecurity := includeAll || insightCategory == "security"
	includeConfiguration := includeAll || insightCategory == "configuration"
	includeNodeResourceChecks := includeNodes
	includePodResourceChecks := includeWorkloads
	includePodScan := includeWorkloads || includeNodeResourceChecks || includeGPUChecks || includeSecurity || includeConfiguration

	nsSet := make(map[string]struct{}, len(nsList))
	for _, ns := range nsList {
		nsSet[ns] = struct{}{}
	}

	type nodeUsage struct {
		RequestCPUMilli int64
		LimitCPUMilli   int64
		RequestMemBytes int64
		LimitMemBytes   int64
		Pods            []string
	}
	type gpuPodRef struct {
		Namespace string
		Name      string
		Requested int64
		Details   []string
	}

	nodeAllocCPU := map[string]int64{}
	nodeAllocMem := map[string]int64{}
	nodeGPUAlloc := map[string]int64{}
	nodeUnschedulable := map[string]bool{}
	usageByNode := map[string]*nodeUsage{}
	gpuNodes := make([]string, 0)
	nodesLoaded := false
	var nodes corev1.NodeList
	if includeNodes || includeGPUChecks {
		loadedNodes, err := c.core.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		nodes = *loadedNodes
		nodesLoaded = true
		nodeAllocCPU = make(map[string]int64, len(nodes.Items))
		nodeAllocMem = make(map[string]int64, len(nodes.Items))
		usageByNode = make(map[string]*nodeUsage, len(nodes.Items))
		for _, node := range nodes.Items {
			nodeAllocCPU[node.Name] = node.Status.Allocatable.Cpu().MilliValue()
			nodeAllocMem[node.Name] = node.Status.Allocatable.Memory().Value()
			nodeUnschedulable[node.Name] = node.Spec.Unschedulable
			usageByNode[node.Name] = &nodeUsage{}
			gpuAlloc := node.Status.Allocatable[corev1.ResourceName("nvidia.com/gpu")]
			if gpuAlloc.Value() > 0 {
				nodeGPUAlloc[node.Name] = gpuAlloc.Value()
				gpuNodes = append(gpuNodes, node.Name)
			}
		}
	}

	pvcUsageByKey := map[string]pvcVolumeUsage{}
	pvcStatsAvailable := false
	if includeStorage {
		var err error
		pvcUsageByKey, pvcStatsAvailable, err = c.collectPVCVolumeStats(ctx)
		if err != nil {
			return nil, err
		}
	}
	resourceMetrics := resourceMetricsStatus{}
	if includeNodes {
		resourceMetrics = c.resourceMetricsStatusForNodes(ctx, nodes.Items)
	}
	allPodUsageByKey := map[string]usageValues{}
	podMetricsAvailable := false
	if includePodResourceChecks {
		allPodUsageByKey, podMetricsAvailable = c.collectPodUsageMetricsForNamespaces(ctx, nsList, nodes.Items)
	}

	now := time.Now()
	gpuRequestedByNode := map[string]int64{}
	gpuRequestedByNamespace := map[string]int64{}
	pendingGPUPods := make([]gpuPodRef, 0)
	nonGPUPodsOnGPUNode := map[string][]string{}
	ingressRouteRefs := map[string][]string{}
	ingressRouteLabels := map[string]string{}

	alerts := make([]InsightAlert, 0)
	if includeNodes && !resourceMetrics.metricsServerAvailable {
		details := []string{"API metrics.k8s.io/v1beta1 is unavailable."}
		severity := "warning"
		summary := "metrics-server is unavailable. BeaverDeck is using direct kubelet /metrics/resource scraping instead."
		if resourceMetrics.directAvailable {
			details = append(details, "Fallback to kubelet /metrics/resource is active.")
			details = append(details, "CPU usage derived from kubelet counters needs one previous scrape to stabilize.")
		} else {
			severity = "critical"
			summary = "metrics-server is unavailable and direct kubelet /metrics/resource scraping also failed."
			details = append(details, "Fallback to kubelet /metrics/resource is unavailable.")
		}
		alerts = append(alerts, InsightAlert{
			Key:        "cluster-metrics-server",
			CheckType:  "metrics-pipeline",
			CheckLabel: "Metrics Pipeline",
			Status:     "alert",
			Category:   "Observability",
			Severity:   severity,
			Title:      "metrics-server is unavailable",
			Summary:    summary,
			Details:    details,
		})
	}
	if includeNodes && nodesLoaded {
		for _, node := range nodes.Items {
			conditionProblems := nodeConditionProblems(&node)
			if len(conditionProblems) > 0 {
				alerts = append(alerts, InsightAlert{
					Key:          fmt.Sprintf("node-condition:%s", node.Name),
					CheckType:    "node-condition",
					CheckLabel:   "Node Conditions",
					Status:       "alert",
					Category:     "Nodes",
					Severity:     "critical",
					Title:        fmt.Sprintf("Node %s reports unhealthy conditions", node.Name),
					Summary:      "Node readiness or pressure conditions require attention.",
					ResourceKind: "Node",
					ResourceName: node.Name,
					Node:         node.Name,
					Details:      conditionProblems,
				})
				continue
			}
			alerts = append(alerts, InsightAlert{
				Key:          fmt.Sprintf("node-condition:%s", node.Name),
				CheckType:    "node-condition",
				CheckLabel:   "Node Conditions",
				Status:       "ok",
				Category:     "Nodes",
				Severity:     "ok",
				Title:        fmt.Sprintf("Node %s is healthy", node.Name),
				Summary:      "Node is Ready and no pressure conditions are active.",
				ResourceKind: "Node",
				ResourceName: node.Name,
				Node:         node.Name,
			})
		}
	}

	for _, ns := range nsList {
		activeSecurityPodCount := 0
		var secretByName map[string]corev1.Secret
		secretsLoaded := false
		loadSecretsByName := func() (map[string]corev1.Secret, error) {
			if secretsLoaded {
				return secretByName, nil
			}
			secrets, err := c.core.CoreV1().Secrets(ns).List(ctx, metav1.ListOptions{})
			if err != nil {
				return nil, err
			}
			secretByName = make(map[string]corev1.Secret, len(secrets.Items))
			for _, secret := range secrets.Items {
				secretByName[secret.Name] = secret
			}
			secretsLoaded = true
			return secretByName, nil
		}

		if includeStorage {
			pvcs, err := c.core.CoreV1().PersistentVolumeClaims(ns).List(ctx, metav1.ListOptions{})
			if err != nil {
				return nil, err
			}
			for _, pvc := range pvcs.Items {
				if pvc.Status.Phase != corev1.ClaimBound || strings.TrimSpace(pvc.Spec.VolumeName) == "" {
					alerts = append(alerts, InsightAlert{
						Key:          fmt.Sprintf("pvc-binding:%s:%s", ns, pvc.Name),
						CheckType:    "pvc-binding",
						CheckLabel:   "PVC Binding",
						Status:       "alert",
						Category:     "Storage",
						Severity:     "critical",
						Title:        fmt.Sprintf("PVC %s/%s is not bound", ns, pvc.Name),
						Summary:      "PersistentVolumeClaim exists but no volume has been provisioned or bound yet.",
						Namespace:    ns,
						ResourceKind: "PersistentVolumeClaim",
						ResourceName: pvc.Name,
						Details: []string{
							fmt.Sprintf("Phase: %s", pvc.Status.Phase),
							fmt.Sprintf("Volume: %s", strings.TrimSpace(pvc.Spec.VolumeName)),
						},
					})
				} else {
					alerts = append(alerts, InsightAlert{
						Key:          fmt.Sprintf("pvc-binding:%s:%s", ns, pvc.Name),
						CheckType:    "pvc-binding",
						CheckLabel:   "PVC Binding",
						Status:       "ok",
						Category:     "Storage",
						Severity:     "ok",
						Title:        fmt.Sprintf("PVC %s/%s is bound", ns, pvc.Name),
						Summary:      "PersistentVolumeClaim is bound to a backing volume.",
						Namespace:    ns,
						ResourceKind: "PersistentVolumeClaim",
						ResourceName: pvc.Name,
						Details:      []string{fmt.Sprintf("Volume: %s", pvc.Spec.VolumeName)},
					})
				}

				usage := pvcUsageByKey[ns+"/"+pvc.Name]
				if !pvcStatsAvailable || usage.CapacityBytes <= 0 {
					continue
				}
				usedPct := float64(usage.UsedBytes) / float64(usage.CapacityBytes)
				details := []string{
					fmt.Sprintf("Used: %s / %s", formatBytesIEC(usage.UsedBytes), formatBytesIEC(usage.CapacityBytes)),
					fmt.Sprintf("Available: %s", formatBytesIEC(max64(usage.AvailableBytes, 0))),
					fmt.Sprintf("Usage: %.1f%%", usedPct*100),
				}
				if usedPct >= 0.85 {
					alerts = append(alerts, InsightAlert{
						Key:          fmt.Sprintf("pvc-almost-full:%s:%s", ns, pvc.Name),
						CheckType:    "pvc-usage",
						CheckLabel:   "PVC Usage",
						Status:       "alert",
						Category:     "Storage",
						Severity:     "warning",
						Title:        fmt.Sprintf("PVC %s/%s is almost full", ns, pvc.Name),
						Summary:      "PersistentVolumeClaim usage is above 85% of available capacity.",
						Namespace:    ns,
						ResourceKind: "PersistentVolumeClaim",
						ResourceName: pvc.Name,
						Details:      details,
					})
				} else {
					alerts = append(alerts, InsightAlert{
						Key:          fmt.Sprintf("pvc-almost-full:%s:%s", ns, pvc.Name),
						CheckType:    "pvc-usage",
						CheckLabel:   "PVC Usage",
						Status:       "ok",
						Category:     "Storage",
						Severity:     "ok",
						Title:        fmt.Sprintf("PVC %s/%s has healthy free space", ns, pvc.Name),
						Summary:      "PersistentVolumeClaim usage is below the 85% warning threshold.",
						Namespace:    ns,
						ResourceKind: "PersistentVolumeClaim",
						ResourceName: pvc.Name,
						Details:      details,
					})
				}
			}

		}

		if includePodScan {
			eventsByPod := map[string][]corev1.Event{}
			secretNames := map[string]struct{}{}
			configMapNames := map[string]struct{}{}
			namespaceHasGPUQuota := false
			if includeWorkloads || includeGPUChecks {
				events, err := c.core.CoreV1().Events(ns).List(ctx, metav1.ListOptions{})
				if err != nil {
					return nil, err
				}
				for _, event := range events.Items {
					if event.InvolvedObject.Kind != "Pod" || event.InvolvedObject.Name == "" {
						continue
					}
					eventsByPod[event.InvolvedObject.Name] = append(eventsByPod[event.InvolvedObject.Name], event)
				}
			}

			if includeConfiguration {
				secrets, err := loadSecretsByName()
				if err != nil {
					return nil, err
				}
				for secretName := range secrets {
					secretNames[secretName] = struct{}{}
				}

				configMaps, err := c.core.CoreV1().ConfigMaps(ns).List(ctx, metav1.ListOptions{})
				if err != nil {
					return nil, err
				}
				for _, configMap := range configMaps.Items {
					configMapNames[configMap.Name] = struct{}{}
				}
			}

			if includeGPUChecks {
				quotas, err := c.core.CoreV1().ResourceQuotas(ns).List(ctx, metav1.ListOptions{})
				if err != nil {
					return nil, err
				}
				namespaceHasGPUQuota = namespaceGPUQuotaPresent(quotas.Items)
			}

			if includeWorkloads {
				daemonSets, err := c.core.AppsV1().DaemonSets(ns).List(ctx, metav1.ListOptions{})
				if err != nil {
					return nil, err
				}
				for _, daemonSet := range daemonSets.Items {
					alerts = append(alerts, daemonSetReadinessInsight(ns, &daemonSet))
				}
			}

			pods, err := c.core.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
			if err != nil {
				return nil, err
			}
			for _, pod := range pods.Items {
				podRef := fmt.Sprintf("%s/%s", ns, pod.Name)
				activePod := podActive(&pod)
				gpuRequestCount := podGPURequestCount(&pod)
				if includeGPUChecks && gpuRequestCount > 0 && activePod {
					gpuRequestedByNamespace[ns] += gpuRequestCount
					if pod.Spec.NodeName != "" {
						gpuRequestedByNode[pod.Spec.NodeName] += gpuRequestCount
					}
				}
				if includeGPUChecks && gpuRequestCount == 0 && activePod && pod.Spec.NodeName != "" && nodeGPUAlloc[pod.Spec.NodeName] > 0 && !podOwnedByKind(&pod, "DaemonSet") {
					nonGPUPodsOnGPUNode[pod.Spec.NodeName] = append(nonGPUPodsOnGPUNode[pod.Spec.NodeName], podRef)
				}

				if includeNodeResourceChecks && pod.Spec.NodeName != "" && pod.Status.Phase != corev1.PodSucceeded && pod.Status.Phase != corev1.PodFailed {
					reqCPU, limCPU, reqMem, limMem := podResourceTotals(&pod)
					usage := usageByNode[pod.Spec.NodeName]
					if usage == nil {
						usage = &nodeUsage{}
						usageByNode[pod.Spec.NodeName] = usage
					}
					usage.RequestCPUMilli += reqCPU
					usage.LimitCPUMilli += limCPU
					usage.RequestMemBytes += reqMem
					usage.LimitMemBytes += limMem
					usage.Pods = append(usage.Pods, fmt.Sprintf("%s/%s", ns, pod.Name))
				}

				if includeWorkloads && pod.Status.Phase == corev1.PodPending && now.Sub(pod.CreationTimestamp.Time) >= pendingPodAlertAfter {
					details := []string{
						fmt.Sprintf("Pending for: %s", age(pod.CreationTimestamp.Time)),
						fmt.Sprintf("Node: %s", firstInsightValue(pod.Spec.NodeName, "-")),
					}
					details = append(details, eventDetails(eventsByPod[pod.Name], "FailedScheduling", "FailedMount", "FailedAttachVolume", "BackOff", "ErrImagePull", "ImagePullBackOff")...)
					alerts = append(alerts, InsightAlert{
						Key:          fmt.Sprintf("pod-pending:%s:%s", ns, pod.Name),
						CheckType:    "pod-pending",
						CheckLabel:   "Pod Pending",
						Status:       "alert",
						Category:     "Scheduling",
						Severity:     "warning",
						Title:        fmt.Sprintf("Pod %s/%s has been pending for %s", ns, pod.Name, age(pod.CreationTimestamp.Time)),
						Summary:      "Pod has remained Pending longer than the scheduling threshold.",
						Namespace:    ns,
						ResourceKind: "Pod",
						ResourceName: pod.Name,
						Details:      details,
					})
				}

				if includeWorkloads {
					waitingProblems := podWaitingReasonDetails(&pod, map[string]struct{}{
						"CrashLoopBackOff": {},
						"ImagePullBackOff": {},
						"ErrImagePull":     {},
					})
					if len(waitingProblems) > 0 {
						alerts = append(alerts, InsightAlert{
							Key:          fmt.Sprintf("pod-container-waiting:%s:%s", ns, pod.Name),
							CheckType:    "container-waiting",
							CheckLabel:   "Container Waiting",
							Status:       "alert",
							Category:     "Workloads",
							Severity:     "warning",
							Title:        fmt.Sprintf("Pod %s/%s has containers waiting", ns, pod.Name),
							Summary:      "At least one container is in CrashLoopBackOff or image pull backoff.",
							Namespace:    ns,
							ResourceKind: "Pod",
							ResourceName: pod.Name,
							Details:      waitingProblems,
						})
					}

					oomKilled := podOOMKilledDetails(&pod)
					if len(oomKilled) > 0 {
						alerts = append(alerts, InsightAlert{
							Key:          fmt.Sprintf("pod-oomkilled:%s:%s", ns, pod.Name),
							CheckType:    "oomkilled",
							CheckLabel:   "OOMKilled",
							Status:       "alert",
							Category:     "Workloads",
							Severity:     "warning",
							Title:        fmt.Sprintf("Pod %s/%s has OOMKilled containers", ns, pod.Name),
							Summary:      "Container memory limits or workload memory behavior may need review.",
							Namespace:    ns,
							ResourceKind: "Pod",
							ResourceName: pod.Name,
							Details:      oomKilled,
						})
					}

					restarts, restartDetails := podRestartDetails(&pod)
					if restarts >= highRestartThreshold {
						alerts = append(alerts, InsightAlert{
							Key:          fmt.Sprintf("pod-high-restarts:%s:%s", ns, pod.Name),
							CheckType:    "pod-restarts",
							CheckLabel:   "Pod Restarts",
							Status:       "alert",
							Category:     "Workloads",
							Severity:     "warning",
							Title:        fmt.Sprintf("Pod %s/%s has %d restarts", ns, pod.Name, restarts),
							Summary:      "Container restart count is above the alert threshold.",
							Namespace:    ns,
							ResourceKind: "Pod",
							ResourceName: pod.Name,
							Details:      restartDetails,
						})
					}
				}

				var missingRequests []string
				if includePodResourceChecks || includeGPUChecks {
					missingRequests = podMissingResourceRequestDetails(&pod)
				}
				if includePodResourceChecks && activePod && len(missingRequests) > 0 {
					alerts = append(alerts, InsightAlert{
						Key:          fmt.Sprintf("pod-missing-requests:%s:%s", ns, pod.Name),
						CheckType:    "missing-requests",
						CheckLabel:   "Resource Requests",
						Status:       "alert",
						Category:     "Workloads",
						Severity:     "warning",
						Title:        fmt.Sprintf("Pod %s/%s has containers without CPU or memory requests", ns, pod.Name),
						Summary:      "Missing requests reduce scheduling and capacity planning accuracy.",
						Namespace:    ns,
						ResourceKind: "Pod",
						ResourceName: pod.Name,
						Details:      missingRequests,
					})
				}

				if includeSecurity {
					if activePod {
						activeSecurityPodCount++
					}
					securityProblems := podPrivilegeDetails(&pod)
					if activePod && len(securityProblems) > 0 {
						alerts = append(alerts, InsightAlert{
							Key:          fmt.Sprintf("pod-privileged:%s:%s", ns, pod.Name),
							CheckType:    "pod-privileged",
							CheckLabel:   "Pod Privileges",
							Status:       "alert",
							Category:     "Security",
							Severity:     "warning",
							Title:        fmt.Sprintf("Pod %s/%s uses elevated privileges", ns, pod.Name),
							Summary:      "Pod uses privileged containers or host namespaces.",
							Namespace:    ns,
							ResourceKind: "Pod",
							ResourceName: pod.Name,
							Details:      securityProblems,
						})
					}
					sensitiveEnvVars := podSensitiveLiteralEnvDetails(&pod)
					if activePod && len(sensitiveEnvVars) > 0 {
						alerts = append(alerts, InsightAlert{
							Key:          fmt.Sprintf("pod-sensitive-env:%s:%s", ns, pod.Name),
							CheckType:    "sensitive-env-literal",
							CheckLabel:   "Sensitive Env Vars",
							Status:       "alert",
							Category:     "Security",
							Severity:     "warning",
							Title:        fmt.Sprintf("Pod %s/%s defines sensitive literal environment variables", ns, pod.Name),
							Summary:      "Sensitive values should be referenced from Secrets or identity providers instead of literal pod environment values.",
							Namespace:    ns,
							ResourceKind: "Pod",
							ResourceName: pod.Name,
							Details:      sensitiveEnvVars,
						})
					}
				}

				if includeConfiguration {
					refProblems := podReferenceProblems(&pod, secretNames, configMapNames)
					if activePod && len(refProblems) > 0 {
						alerts = append(alerts, InsightAlert{
							Key:          fmt.Sprintf("pod-missing-refs:%s:%s", ns, pod.Name),
							CheckType:    "missing-references",
							CheckLabel:   "Missing References",
							Status:       "alert",
							Category:     "Configuration",
							Severity:     "critical",
							Title:        fmt.Sprintf("Pod %s/%s references missing Secrets or ConfigMaps", ns, pod.Name),
							Summary:      "Required Secret or ConfigMap references could prevent containers from starting or mounting configuration.",
							Namespace:    ns,
							ResourceKind: "Pod",
							ResourceName: pod.Name,
							Details:      refProblems,
						})
					}
				}

				if includeGPUChecks && activePod && gpuRequestCount > 0 {
					if pod.Status.Phase == corev1.PodPending && now.Sub(pod.CreationTimestamp.Time) >= gpuPendingPodAlertAfter {
						details := []string{
							fmt.Sprintf("Requested GPUs: %d", gpuRequestCount),
							fmt.Sprintf("Pending for: %s", age(pod.CreationTimestamp.Time)),
						}
						details = append(details, eventDetails(eventsByPod[pod.Name], "FailedScheduling", "NotTriggerScaleUp", "FailedMount")...)
						pendingGPUPods = append(pendingGPUPods, gpuPodRef{
							Namespace: ns,
							Name:      pod.Name,
							Requested: gpuRequestCount,
							Details:   details,
						})
						alerts = append(alerts, InsightAlert{
							Key:          fmt.Sprintf("gpu-pod-pending:%s:%s", ns, pod.Name),
							CheckType:    "gpu-pod-pending",
							CheckLabel:   "GPU Pod Pending",
							Status:       "alert",
							Category:     "GPU",
							Severity:     "warning",
							Title:        fmt.Sprintf("GPU pod %s/%s is pending", ns, pod.Name),
							Summary:      "Pod requests GPU resources but has not been scheduled.",
							Namespace:    ns,
							ResourceKind: "Pod",
							ResourceName: pod.Name,
							Details:      details,
						})
					}
					if pod.Spec.NodeName != "" && !podReady(&pod) && now.Sub(podUnreadySince(&pod)) >= gpuUnreadyPodAlertAfter {
						details := []string{
							fmt.Sprintf("Requested GPUs: %d", gpuRequestCount),
							fmt.Sprintf("Node: %s", pod.Spec.NodeName),
							fmt.Sprintf("Unready for: %s", age(podUnreadySince(&pod))),
						}
						details = append(details, podWaitingReasonDetails(&pod, nil)...)
						alerts = append(alerts, InsightAlert{
							Key:          fmt.Sprintf("gpu-pod-unready:%s:%s", ns, pod.Name),
							CheckType:    "gpu-pod-unready",
							CheckLabel:   "GPU Pod Readiness",
							Status:       "alert",
							Category:     "GPU",
							Severity:     "warning",
							Title:        fmt.Sprintf("GPU pod %s/%s is scheduled but not ready", ns, pod.Name),
							Summary:      "GPU resources are allocated to a pod that is not ready.",
							Namespace:    ns,
							ResourceKind: "Pod",
							ResourceName: pod.Name,
							Node:         pod.Spec.NodeName,
							Details:      details,
						})
					}
					if len(missingRequests) > 0 {
						alerts = append(alerts, InsightAlert{
							Key:          fmt.Sprintf("gpu-pod-missing-requests:%s:%s", ns, pod.Name),
							CheckType:    "gpu-pod-requests",
							CheckLabel:   "GPU Pod Requests",
							Status:       "alert",
							Category:     "GPU",
							Severity:     "warning",
							Title:        fmt.Sprintf("GPU pod %s/%s lacks CPU or memory requests", ns, pod.Name),
							Summary:      "GPU workloads should declare CPU and memory requests so scheduler and capacity signals stay meaningful.",
							Namespace:    ns,
							ResourceKind: "Pod",
							ResourceName: pod.Name,
							Details:      missingRequests,
						})
					}
				}

				if includeSecurity {
					rootContexts := podRootContexts(&pod)
					if len(rootContexts) > 0 {
						alerts = append(alerts, InsightAlert{
							Key:          fmt.Sprintf("root-pod:%s:%s", ns, pod.Name),
							CheckType:    "root-user",
							CheckLabel:   "Root User",
							Status:       "alert",
							Category:     "Security",
							Severity:     "warning",
							Title:        fmt.Sprintf("Pod %s/%s runs as root", ns, pod.Name),
							Summary:      "Security context explicitly uses UID 0.",
							Namespace:    ns,
							ResourceKind: "Pod",
							ResourceName: pod.Name,
							Details:      rootContexts,
						})
					} else {
						alerts = append(alerts, InsightAlert{
							Key:          fmt.Sprintf("root-pod:%s:%s", ns, pod.Name),
							CheckType:    "root-user",
							CheckLabel:   "Root User",
							Status:       "ok",
							Category:     "Security",
							Severity:     "ok",
							Title:        fmt.Sprintf("Pod %s/%s does not run as root", ns, pod.Name),
							Summary:      "No explicit UID 0 usage was detected in the pod security context.",
							Namespace:    ns,
							ResourceKind: "Pod",
							ResourceName: pod.Name,
						})
					}
				}

				if includePodResourceChecks && strings.EqualFold(string(pod.Status.Phase), string(corev1.PodRunning)) && podMetricsAvailable {
					usage, usageFound := allPodUsageByKey[ns+"/"+pod.Name]
					if !usageFound {
						continue
					}
					reqCPU, _, reqMem, _ := podResourceTotals(&pod)
					details := make([]string, 0, 2)
					if reqCPU > 0 && usage.cpuMilli > reqCPU {
						details = append(details, fmt.Sprintf("CPU used %dm is above request %dm", usage.cpuMilli, reqCPU))
					}
					if reqMem > 0 && usage.memoryBytes > reqMem {
						details = append(details, fmt.Sprintf("Memory used %s is above request %s", formatBytesIEC(usage.memoryBytes), formatBytesIEC(reqMem)))
					}
					if len(details) > 0 {
						alerts = append(alerts, InsightAlert{
							Key:          fmt.Sprintf("pod-over-request:%s:%s", ns, pod.Name),
							CheckType:    "pod-request-usage",
							CheckLabel:   "Pod Request Usage",
							Status:       "alert",
							Category:     "Workloads",
							Severity:     "warning",
							Title:        fmt.Sprintf("Pod %s/%s is using more than requested", ns, pod.Name),
							Summary:      "Current CPU or memory usage is above the pod request.",
							Namespace:    ns,
							ResourceKind: "Pod",
							ResourceName: pod.Name,
							Details:      details,
						})
					} else if reqCPU > 0 || reqMem > 0 {
						alerts = append(alerts, InsightAlert{
							Key:          fmt.Sprintf("pod-over-request:%s:%s", ns, pod.Name),
							CheckType:    "pod-request-usage",
							CheckLabel:   "Pod Request Usage",
							Status:       "ok",
							Category:     "Workloads",
							Severity:     "ok",
							Title:        fmt.Sprintf("Pod %s/%s stays within requests", ns, pod.Name),
							Summary:      "Current CPU and memory usage stay within the requested resources.",
							Namespace:    ns,
							ResourceKind: "Pod",
							ResourceName: pod.Name,
						})
					}

					if now.Sub(pod.CreationTimestamp.Time) >= underRequestedPodAfter {
						underRequestDetails := podUnderRequestDetails(reqCPU, reqMem, usage)
						if len(underRequestDetails) > 0 {
							alerts = append(alerts, InsightAlert{
								Key:          fmt.Sprintf("pod-under-request:%s:%s", ns, pod.Name),
								CheckType:    "pod-under-request",
								CheckLabel:   "Overrequested Pod",
								Status:       "alert",
								Category:     "Workloads",
								Severity:     "warning",
								Title:        fmt.Sprintf("Pod %s/%s is using far less than requested", ns, pod.Name),
								Summary:      "Current usage is well below requested CPU or memory.",
								Namespace:    ns,
								ResourceKind: "Pod",
								ResourceName: pod.Name,
								Details:      underRequestDetails,
							})
						}
					}
				}
			}

			if includeSecurity && activeSecurityPodCount > 0 {
				networkPolicies, err := c.core.NetworkingV1().NetworkPolicies(ns).List(ctx, metav1.ListOptions{})
				if err != nil {
					return nil, err
				}
				details := []string{
					fmt.Sprintf("Active pods in selected namespace: %d", activeSecurityPodCount),
					fmt.Sprintf("NetworkPolicies: %d", len(networkPolicies.Items)),
				}
				if len(networkPolicies.Items) == 0 {
					alerts = append(alerts, InsightAlert{
						Key:        fmt.Sprintf("network-policy-missing:%s", ns),
						CheckType:  "network-policy-coverage",
						CheckLabel: "NetworkPolicy Coverage",
						Status:     "alert",
						Category:   "Security",
						Severity:   "warning",
						Title:      fmt.Sprintf("Namespace %s has pods without any NetworkPolicy", ns),
						Summary:    "Namespace has active pods, but no NetworkPolicy objects were found to define pod traffic boundaries.",
						Namespace:  ns,
						Details:    details,
					})
				} else {
					alerts = append(alerts, InsightAlert{
						Key:        fmt.Sprintf("network-policy-missing:%s", ns),
						CheckType:  "network-policy-coverage",
						CheckLabel: "NetworkPolicy Coverage",
						Status:     "ok",
						Category:   "Security",
						Severity:   "ok",
						Title:      fmt.Sprintf("Namespace %s has NetworkPolicy objects", ns),
						Summary:    "Namespace has at least one NetworkPolicy object.",
						Namespace:  ns,
						Details:    details,
					})
				}
			}

			if includeGPUChecks && gpuRequestedByNamespace[ns] > 0 {
				details := []string{
					fmt.Sprintf("Requested GPUs in namespace: %d", gpuRequestedByNamespace[ns]),
					fmt.Sprintf("GPU quota configured: %t", namespaceHasGPUQuota),
				}
				alerts = append(alerts, InsightAlert{
					Key:        fmt.Sprintf("gpu-namespace-usage:%s", ns),
					CheckType:  "gpu-namespace-usage",
					CheckLabel: "Namespace GPU Usage",
					Status:     "ok",
					Category:   "GPU",
					Severity:   "ok",
					Title:      fmt.Sprintf("Namespace %s requests %d GPU", ns, gpuRequestedByNamespace[ns]),
					Summary:    "Namespace has active GPU workload requests.",
					Namespace:  ns,
					Details:    details,
				})
				if !namespaceHasGPUQuota {
					alerts = append(alerts, InsightAlert{
						Key:        fmt.Sprintf("gpu-quota-missing:%s", ns),
						CheckType:  "gpu-quota",
						CheckLabel: "GPU Quota",
						Status:     "alert",
						Category:   "GPU",
						Severity:   "warning",
						Title:      fmt.Sprintf("Namespace %s uses GPUs without a ResourceQuota", ns),
						Summary:    "GPU-consuming namespaces should usually have an explicit ResourceQuota for GPU resources.",
						Namespace:  ns,
						Details:    details,
					})
				}
			}
		}

		if includeNetworking {
			services, err := c.core.CoreV1().Services(ns).List(ctx, metav1.ListOptions{})
			if err != nil {
				return nil, err
			}
			serviceNames := make(map[string]struct{}, len(services.Items))
			for _, service := range services.Items {
				serviceNames[service.Name] = struct{}{}
			}
			endpointSlices, err := c.core.DiscoveryV1().EndpointSlices(ns).List(ctx, metav1.ListOptions{})
			if err != nil {
				return nil, err
			}
			readyEndpointsByService := make(map[string]int, len(services.Items))
			totalEndpointsByService := make(map[string]int, len(services.Items))
			for _, slice := range endpointSlices.Items {
				svcName := strings.TrimSpace(slice.Labels[discoveryv1.LabelServiceName])
				if svcName == "" {
					continue
				}
				for _, endpoint := range slice.Endpoints {
					totalEndpointsByService[svcName]++
					if endpoint.Conditions.Ready == nil || *endpoint.Conditions.Ready {
						readyEndpointsByService[svcName]++
					}
				}
			}
			for _, service := range services.Items {
				if service.Spec.Type == corev1.ServiceTypeLoadBalancer {
					details := []string{
						fmt.Sprintf("Type: %s", service.Spec.Type),
						fmt.Sprintf("External ingress entries: %d", len(service.Status.LoadBalancer.Ingress)),
					}
					if len(service.Status.LoadBalancer.Ingress) == 0 {
						alerts = append(alerts, InsightAlert{
							Key:          fmt.Sprintf("loadbalancer-pending:%s:%s", ns, service.Name),
							CheckType:    "loadbalancer-pending",
							CheckLabel:   "LoadBalancer",
							Status:       "alert",
							Category:     "Reachability",
							Severity:     "warning",
							Title:        fmt.Sprintf("LoadBalancer Service %s/%s has no external address", ns, service.Name),
							Summary:      "Service is type LoadBalancer but Kubernetes has not assigned an external ingress address.",
							Namespace:    ns,
							ResourceKind: "Service",
							ResourceName: service.Name,
							Details:      details,
						})
					} else {
						alerts = append(alerts, InsightAlert{
							Key:          fmt.Sprintf("loadbalancer-pending:%s:%s", ns, service.Name),
							CheckType:    "loadbalancer-pending",
							CheckLabel:   "LoadBalancer",
							Status:       "ok",
							Category:     "Reachability",
							Severity:     "ok",
							Title:        fmt.Sprintf("LoadBalancer Service %s/%s has an external address", ns, service.Name),
							Summary:      "Service has at least one external ingress address.",
							Namespace:    ns,
							ResourceKind: "Service",
							ResourceName: service.Name,
							Details:      details,
						})
					}
				}
				if service.Spec.Type == corev1.ServiceTypeExternalName || len(service.Spec.Selector) == 0 {
					alerts = append(alerts, InsightAlert{
						Key:          fmt.Sprintf("service-endpoints:%s:%s", ns, service.Name),
						CheckType:    "service-endpoints",
						CheckLabel:   "Service Endpoints",
						Status:       "ok",
						Category:     "Reachability",
						Severity:     "ok",
						Title:        fmt.Sprintf("Service %s/%s does not require pod endpoints", ns, service.Name),
						Summary:      "Service is ExternalName or uses manual endpoint management, so pod-backed endpoint checks are skipped.",
						Namespace:    ns,
						ResourceKind: "Service",
						ResourceName: service.Name,
					})
					continue
				}
				readyCount := readyEndpointsByService[service.Name]
				totalCount := totalEndpointsByService[service.Name]
				details := []string{
					fmt.Sprintf("Selector: %s", formatSelector(service.Spec.Selector)),
					fmt.Sprintf("Ready endpoints: %d", readyCount),
					fmt.Sprintf("Observed endpoints: %d", totalCount),
				}
				if readyCount == 0 {
					alerts = append(alerts, InsightAlert{
						Key:          fmt.Sprintf("service-endpoints:%s:%s", ns, service.Name),
						CheckType:    "service-endpoints",
						CheckLabel:   "Service Endpoints",
						Status:       "alert",
						Category:     "Reachability",
						Severity:     "warning",
						Title:        fmt.Sprintf("Service %s/%s has no ready endpoints", ns, service.Name),
						Summary:      "Traffic routed to this Service will not reach any ready pod backends.",
						Namespace:    ns,
						ResourceKind: "Service",
						ResourceName: service.Name,
						Details:      details,
					})
					continue
				}
				alerts = append(alerts, InsightAlert{
					Key:          fmt.Sprintf("service-endpoints:%s:%s", ns, service.Name),
					CheckType:    "service-endpoints",
					CheckLabel:   "Service Endpoints",
					Status:       "ok",
					Category:     "Reachability",
					Severity:     "ok",
					Title:        fmt.Sprintf("Service %s/%s has ready endpoints", ns, service.Name),
					Summary:      "Service routes to at least one ready pod backend.",
					Namespace:    ns,
					ResourceKind: "Service",
					ResourceName: service.Name,
					Details:      details,
				})
			}

			ingresses, err := c.core.NetworkingV1().Ingresses(ns).List(ctx, metav1.ListOptions{})
			if err != nil {
				return nil, err
			}
			ingressNeedsSecretLookup := false
			for _, ingress := range ingresses.Items {
				for _, tls := range ingress.Spec.TLS {
					if strings.TrimSpace(tls.SecretName) != "" {
						ingressNeedsSecretLookup = true
						break
					}
				}
				if ingressNeedsSecretLookup {
					break
				}
			}
			networkSecretByName := map[string]corev1.Secret{}
			if ingressNeedsSecretLookup {
				var err error
				networkSecretByName, err = loadSecretsByName()
				if err != nil {
					return nil, err
				}
			}
			for _, ingress := range ingresses.Items {
				for routeKey, routeLabel := range ingressRouteKeys(&ingress) {
					ingressRouteRefs[routeKey] = append(ingressRouteRefs[routeKey], fmt.Sprintf("%s/%s", ns, ingress.Name))
					ingressRouteLabels[routeKey] = routeLabel
				}
				tlsProblems := make([]string, 0)
				for _, tls := range ingress.Spec.TLS {
					if strings.TrimSpace(tls.SecretName) == "" {
						tlsProblems = append(tlsProblems, "TLS entry has no secretName")
						continue
					}
					secret, ok := networkSecretByName[tls.SecretName]
					if !ok {
						tlsProblems = append(tlsProblems, fmt.Sprintf("Secret %s is missing", tls.SecretName))
						continue
					}
					if secret.Type != corev1.SecretTypeTLS {
						tlsProblems = append(tlsProblems, fmt.Sprintf("Secret %s has type %s instead of kubernetes.io/tls", tls.SecretName, secret.Type))
					}
					if len(secret.Data[corev1.TLSCertKey]) == 0 || len(secret.Data[corev1.TLSPrivateKeyKey]) == 0 {
						tlsProblems = append(tlsProblems, fmt.Sprintf("Secret %s does not contain tls.crt/tls.key", tls.SecretName))
					}
				}
				if len(ingress.Spec.TLS) > 0 {
					if len(tlsProblems) > 0 {
						alerts = append(alerts, InsightAlert{
							Key:          fmt.Sprintf("ingress-tls:%s:%s", ns, ingress.Name),
							CheckType:    "ingress-tls",
							CheckLabel:   "Ingress TLS",
							Status:       "alert",
							Category:     "Reachability",
							Severity:     "critical",
							Title:        fmt.Sprintf("Ingress %s/%s has invalid TLS configuration", ns, ingress.Name),
							Summary:      "Ingress enables TLS, but the referenced certificate secret is missing or invalid.",
							Namespace:    ns,
							ResourceKind: "Ingress",
							ResourceName: ingress.Name,
							Details:      tlsProblems,
						})
					} else {
						alerts = append(alerts, InsightAlert{
							Key:          fmt.Sprintf("ingress-tls:%s:%s", ns, ingress.Name),
							CheckType:    "ingress-tls",
							CheckLabel:   "Ingress TLS",
							Status:       "ok",
							Category:     "Reachability",
							Severity:     "ok",
							Title:        fmt.Sprintf("Ingress %s/%s has valid TLS secret references", ns, ingress.Name),
							Summary:      "All TLS entries point to existing kubernetes.io/tls secrets.",
							Namespace:    ns,
							ResourceKind: "Ingress",
							ResourceName: ingress.Name,
						})
					}
				}

				backendProblems := make([]string, 0)
				for _, svcName := range ingressBackendServiceNames(&ingress) {
					if _, ok := serviceNames[svcName]; !ok {
						backendProblems = append(backendProblems, fmt.Sprintf("Service %s does not exist", svcName))
					}
				}
				if len(backendProblems) > 0 {
					alerts = append(alerts, InsightAlert{
						Key:          fmt.Sprintf("ingress-backend:%s:%s", ns, ingress.Name),
						CheckType:    "ingress-backend",
						CheckLabel:   "Ingress Backend",
						Status:       "alert",
						Category:     "Reachability",
						Severity:     "critical",
						Title:        fmt.Sprintf("Ingress %s/%s points to missing backend services", ns, ingress.Name),
						Summary:      "At least one ingress backend references a Service that does not exist.",
						Namespace:    ns,
						ResourceKind: "Ingress",
						ResourceName: ingress.Name,
						Details:      backendProblems,
					})
				} else {
					alerts = append(alerts, InsightAlert{
						Key:          fmt.Sprintf("ingress-backend:%s:%s", ns, ingress.Name),
						CheckType:    "ingress-backend",
						CheckLabel:   "Ingress Backend",
						Status:       "ok",
						Category:     "Reachability",
						Severity:     "ok",
						Title:        fmt.Sprintf("Ingress %s/%s backends resolve to Services", ns, ingress.Name),
						Summary:      "All ingress backends reference existing Services.",
						Namespace:    ns,
						ResourceKind: "Ingress",
						ResourceName: ingress.Name,
					})
				}
			}

			deployments, err := c.core.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{})
			if err != nil {
				return nil, err
			}
			for _, deployment := range deployments.Items {
				if !workloadHasService(deployment.Spec.Template.Labels, services.Items) {
					alerts = append(alerts, InsightAlert{
						Key:          fmt.Sprintf("svc-gap:deployment:%s:%s", ns, deployment.Name),
						CheckType:    "service-coverage",
						CheckLabel:   "Service Coverage",
						Status:       "alert",
						Category:     "Reachability",
						Severity:     "warning",
						Title:        fmt.Sprintf("Deployment %s/%s has no matching Service", ns, deployment.Name),
						Summary:      "No Service selector matches this workload's pod labels.",
						Namespace:    ns,
						ResourceKind: "Deployment",
						ResourceName: deployment.Name,
					})
				} else {
					alerts = append(alerts, InsightAlert{
						Key:          fmt.Sprintf("svc-gap:deployment:%s:%s", ns, deployment.Name),
						CheckType:    "service-coverage",
						CheckLabel:   "Service Coverage",
						Status:       "ok",
						Category:     "Reachability",
						Severity:     "ok",
						Title:        fmt.Sprintf("Deployment %s/%s is covered by a Service", ns, deployment.Name),
						Summary:      "At least one Service selector matches this workload's pod labels.",
						Namespace:    ns,
						ResourceKind: "Deployment",
						ResourceName: deployment.Name,
					})
				}
			}

			statefulSets, err := c.core.AppsV1().StatefulSets(ns).List(ctx, metav1.ListOptions{})
			if err != nil {
				return nil, err
			}
			for _, statefulSet := range statefulSets.Items {
				if !workloadHasService(statefulSet.Spec.Template.Labels, services.Items) {
					alerts = append(alerts, InsightAlert{
						Key:          fmt.Sprintf("svc-gap:statefulset:%s:%s", ns, statefulSet.Name),
						CheckType:    "service-coverage",
						CheckLabel:   "Service Coverage",
						Status:       "alert",
						Category:     "Reachability",
						Severity:     "warning",
						Title:        fmt.Sprintf("StatefulSet %s/%s has no matching Service", ns, statefulSet.Name),
						Summary:      "No Service selector matches this workload's pod labels.",
						Namespace:    ns,
						ResourceKind: "StatefulSet",
						ResourceName: statefulSet.Name,
					})
				} else {
					alerts = append(alerts, InsightAlert{
						Key:          fmt.Sprintf("svc-gap:statefulset:%s:%s", ns, statefulSet.Name),
						CheckType:    "service-coverage",
						CheckLabel:   "Service Coverage",
						Status:       "ok",
						Category:     "Reachability",
						Severity:     "ok",
						Title:        fmt.Sprintf("StatefulSet %s/%s is covered by a Service", ns, statefulSet.Name),
						Summary:      "At least one Service selector matches this workload's pod labels.",
						Namespace:    ns,
						ResourceKind: "StatefulSet",
						ResourceName: statefulSet.Name,
					})
				}
			}
		}
	}

	if includeNetworking {
		for routeKey, refs := range ingressRouteRefs {
			if len(refs) < 2 {
				continue
			}
			sort.Strings(refs)
			alerts = append(alerts, InsightAlert{
				Key:        fmt.Sprintf("ingress-route-collision:%s", routeKey),
				CheckType:  "ingress-route-collision",
				CheckLabel: "Ingress Route Collision",
				Status:     "alert",
				Category:   "Reachability",
				Severity:   "warning",
				Title:      fmt.Sprintf("Ingress route is defined by %d resources", len(refs)),
				Summary:    "Multiple Ingress resources claim the same ingress class, host, and path.",
				Details: append([]string{
					fmt.Sprintf("Route: %s", ingressRouteLabels[routeKey]),
				}, refs...),
			})
		}
	}

	if includeGPUChecks && len(gpuNodes) > 0 {
		totalGPU := int64(0)
		totalRequestedGPU := int64(0)
		totalSchedulableFreeGPU := int64(0)
		for _, nodeName := range gpuNodes {
			alloc := nodeGPUAlloc[nodeName]
			requested := gpuRequestedByNode[nodeName]
			free := max64(alloc-requested, 0)
			totalGPU += alloc
			totalRequestedGPU += requested
			if !nodeUnschedulable[nodeName] {
				totalSchedulableFreeGPU += free
			}

			alerts = append(alerts, gpuAllocationPressureInsight("node", nodeName, alloc, requested))
			if nodeUnschedulable[nodeName] {
				alerts = append(alerts, InsightAlert{
					Key:          fmt.Sprintf("gpu-node-cordoned:%s", nodeName),
					CheckType:    "gpu-node-cordoned",
					CheckLabel:   "GPU Node Scheduling",
					Status:       "alert",
					Category:     "GPU",
					Severity:     "warning",
					Title:        fmt.Sprintf("GPU node %s is cordoned", nodeName),
					Summary:      "Node has GPU capacity but is marked unschedulable.",
					ResourceKind: "Node",
					ResourceName: nodeName,
					Node:         nodeName,
					Details: []string{
						fmt.Sprintf("Allocatable GPUs: %d", alloc),
						fmt.Sprintf("Requested GPUs: %d", requested),
					},
				})
			}
			if requested == 0 {
				alerts = append(alerts, InsightAlert{
					Key:          fmt.Sprintf("gpu-node-idle-allocation:%s", nodeName),
					CheckType:    "gpu-node-idle-allocation",
					CheckLabel:   "GPU Allocation",
					Status:       "alert",
					Category:     "GPU",
					Severity:     "warning",
					Title:        fmt.Sprintf("GPU node %s has no scheduled GPU workloads", nodeName),
					Summary:      "Node advertises GPU capacity, but selected namespaces have no pods requesting GPU on it.",
					ResourceKind: "Node",
					ResourceName: nodeName,
					Node:         nodeName,
					Details: []string{
						fmt.Sprintf("Allocatable GPUs: %d", alloc),
						"GPU utilization requires DCGM metrics; this check only uses Kubernetes scheduling data.",
					},
				})
			}
			if pods := nonGPUPodsOnGPUNode[nodeName]; len(pods) > 0 {
				sort.Strings(pods)
				alerts = append(alerts, InsightAlert{
					Key:          fmt.Sprintf("non-gpu-pods-on-gpu-node:%s", nodeName),
					CheckType:    "non-gpu-pods-on-gpu-node",
					CheckLabel:   "GPU Node Workload Mix",
					Status:       "alert",
					Category:     "GPU",
					Severity:     "warning",
					Title:        fmt.Sprintf("GPU node %s is running non-GPU pods", nodeName),
					Summary:      "Non-DaemonSet pods without GPU requests are scheduled on a GPU node.",
					ResourceKind: "Node",
					ResourceName: nodeName,
					Node:         nodeName,
					Details: append([]string{
						"DaemonSet-managed pods are intentionally ignored.",
					}, limitedDetails(pods, 12)...),
				})
			}
		}
		alerts = append(alerts, gpuAllocationPressureInsight("cluster", "selected namespaces", totalGPU, totalRequestedGPU))

		for _, podRef := range pendingGPUPods {
			if podRef.Requested <= 0 {
				continue
			}
			anyNodeCanFit := false
			for _, nodeName := range gpuNodes {
				if nodeUnschedulable[nodeName] {
					continue
				}
				free := max64(nodeGPUAlloc[nodeName]-gpuRequestedByNode[nodeName], 0)
				if free >= podRef.Requested {
					anyNodeCanFit = true
					break
				}
			}
			if totalSchedulableFreeGPU >= podRef.Requested && !anyNodeCanFit {
				alerts = append(alerts, InsightAlert{
					Key:          fmt.Sprintf("gpu-fragmentation:%s:%s", podRef.Namespace, podRef.Name),
					CheckType:    "gpu-fragmentation",
					CheckLabel:   "GPU Fragmentation",
					Status:       "alert",
					Category:     "GPU",
					Severity:     "warning",
					Title:        fmt.Sprintf("GPU pod %s/%s may be blocked by fragmented capacity", podRef.Namespace, podRef.Name),
					Summary:      "Selected GPU nodes have enough total free GPU capacity, but no single schedulable node can satisfy this pod request.",
					Namespace:    podRef.Namespace,
					ResourceKind: "Pod",
					ResourceName: podRef.Name,
					Details: append([]string{
						fmt.Sprintf("Requested GPUs: %d", podRef.Requested),
						fmt.Sprintf("Total schedulable free GPUs: %d", totalSchedulableFreeGPU),
					}, podRef.Details...),
				})
			}
		}
	}
	if includeGPUChecks && len(gpuNodes) == 0 {
		totalRequestedGPU := int64(0)
		for _, requested := range gpuRequestedByNamespace {
			totalRequestedGPU += requested
		}
		if totalRequestedGPU > 0 {
			alerts = append(alerts, InsightAlert{
				Key:        "gpu-capacity-missing",
				CheckType:  "gpu-capacity-discovery",
				CheckLabel: "GPU Capacity Discovery",
				Status:     "alert",
				Category:   "GPU",
				Severity:   "critical",
				Title:      "GPU workloads are present but no GPU nodes were detected",
				Summary:    "Selected namespaces request GPU resources, but no node advertises allocatable nvidia.com/gpu capacity.",
				Details: []string{
					fmt.Sprintf("Requested GPUs in selected namespaces: %d", totalRequestedGPU),
					"Detected GPU nodes: 0",
				},
			})
		} else {
			alerts = append(alerts, InsightAlert{
				Key:        "gpu-capacity-missing",
				CheckType:  "gpu-capacity-discovery",
				CheckLabel: "GPU Capacity Discovery",
				Status:     "ok",
				Category:   "GPU",
				Severity:   "ok",
				Title:      "No GPU workloads or GPU nodes detected",
				Summary:    "Selected namespaces have no GPU requests and no nodes advertise allocatable nvidia.com/gpu capacity.",
			})
		}
	}

	if includeStorage {
		persistentVolumes, err := c.core.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		for _, pv := range persistentVolumes.Items {
			if pv.Status.Phase != corev1.VolumeReleased {
				continue
			}
			if pv.Spec.ClaimRef != nil && pv.Spec.ClaimRef.Namespace != "" {
				if _, ok := nsSet[pv.Spec.ClaimRef.Namespace]; !ok {
					continue
				}
			}
			releasedAt := pv.CreationTimestamp.Time
			if pv.Status.LastPhaseTransitionTime != nil && !pv.Status.LastPhaseTransitionTime.IsZero() {
				releasedAt = pv.Status.LastPhaseTransitionTime.Time
			}
			releasedFor := time.Since(releasedAt)
			claim := "-"
			if pv.Spec.ClaimRef != nil {
				claim = fmt.Sprintf("%s/%s", pv.Spec.ClaimRef.Namespace, pv.Spec.ClaimRef.Name)
			}
			details := []string{
				fmt.Sprintf("Claim: %s", claim),
				fmt.Sprintf("StorageClass: %s", strings.TrimSpace(pv.Spec.StorageClassName)),
				fmt.Sprintf("Released for: %s", age(releasedAt)),
			}
			if releasedFor > 7*24*time.Hour {
				alerts = append(alerts, InsightAlert{
					Key:          fmt.Sprintf("pv-released:%s", pv.Name),
					CheckType:    "pv-released",
					CheckLabel:   "PV Released",
					Status:       "alert",
					Category:     "Storage",
					Severity:     "warning",
					Title:        fmt.Sprintf("PV %s has been released for more than 7 days", pv.Name),
					Summary:      "Released PersistentVolumes usually indicate storage that is no longer claimed but still needs cleanup or recycling.",
					ResourceKind: "PersistentVolume",
					ResourceName: pv.Name,
					Details:      details,
				})
				continue
			}
			alerts = append(alerts, InsightAlert{
				Key:          fmt.Sprintf("pv-released:%s", pv.Name),
				CheckType:    "pv-released",
				CheckLabel:   "PV Released",
				Status:       "ok",
				Category:     "Storage",
				Severity:     "ok",
				Title:        fmt.Sprintf("PV %s was released recently", pv.Name),
				Summary:      "Released PersistentVolume is still within the 7-day cleanup window.",
				ResourceKind: "PersistentVolume",
				ResourceName: pv.Name,
				Details:      details,
			})
		}
	}

	if includeNodeResourceChecks {
		for nodeName, usage := range usageByNode {
			if usage == nil {
				continue
			}
			allocCPU := nodeAllocCPU[nodeName]
			allocMem := nodeAllocMem[nodeName]
			details := make([]string, 0, 4)
			if usage.RequestCPUMilli > allocCPU {
				details = append(details, fmt.Sprintf("CPU requests: %dm > %dm allocatable", usage.RequestCPUMilli, allocCPU))
			}
			if usage.LimitCPUMilli > allocCPU {
				details = append(details, fmt.Sprintf("CPU limits: %dm > %dm allocatable", usage.LimitCPUMilli, allocCPU))
			}
			if usage.RequestMemBytes > allocMem {
				details = append(details, fmt.Sprintf("Memory requests: %s > %s allocatable", formatBytesIEC(usage.RequestMemBytes), formatBytesIEC(allocMem)))
			}
			if usage.LimitMemBytes > allocMem {
				details = append(details, fmt.Sprintf("Memory limits: %s > %s allocatable", formatBytesIEC(usage.LimitMemBytes), formatBytesIEC(allocMem)))
			}
			if allocCPU > 0 && allocMem > 0 && !nodeUnschedulable[nodeName] {
				cpuRequestPercent := float64(usage.RequestCPUMilli) / float64(allocCPU) * 100
				memRequestPercent := float64(usage.RequestMemBytes) / float64(allocMem) * 100
				utilizationDetails := []string{
					fmt.Sprintf("CPU requests: %dm / %dm allocatable (%.1f%%)", usage.RequestCPUMilli, allocCPU, cpuRequestPercent),
					fmt.Sprintf("Memory requests: %s / %s allocatable (%.1f%%)", formatBytesIEC(usage.RequestMemBytes), formatBytesIEC(allocMem), memRequestPercent),
					fmt.Sprintf("Scheduled pods: %d", len(usage.Pods)),
				}
				if usage.RequestCPUMilli*100 < allocCPU*nodeUnderRequestPercent && usage.RequestMemBytes*100 < allocMem*nodeUnderRequestPercent {
					alerts = append(alerts, InsightAlert{
						Key:          fmt.Sprintf("node-underutilized:%s", nodeName),
						CheckType:    "node-underutilized",
						CheckLabel:   "Node Underutilization",
						Status:       "alert",
						Category:     "Nodes",
						Severity:     "warning",
						Title:        fmt.Sprintf("Node %s has low requested resource usage", nodeName),
						Summary:      "Scheduled pod CPU and memory requests are both below 50% of node allocatable resources.",
						ResourceKind: "Node",
						ResourceName: nodeName,
						Node:         nodeName,
						Details:      utilizationDetails,
					})
				} else {
					alerts = append(alerts, InsightAlert{
						Key:          fmt.Sprintf("node-underutilized:%s", nodeName),
						CheckType:    "node-underutilized",
						CheckLabel:   "Node Underutilization",
						Status:       "ok",
						Category:     "Nodes",
						Severity:     "ok",
						Title:        fmt.Sprintf("Node %s request usage is above the underutilization threshold", nodeName),
						Summary:      "Scheduled pod CPU or memory requests are at least 50% of node allocatable resources.",
						ResourceKind: "Node",
						ResourceName: nodeName,
						Node:         nodeName,
						Details:      utilizationDetails,
					})
				}
			}
			if len(details) == 0 {
				alerts = append(alerts, InsightAlert{
					Key:          fmt.Sprintf("node-overcommit:%s", nodeName),
					CheckType:    "node-capacity",
					CheckLabel:   "Node Capacity",
					Status:       "ok",
					Category:     "Nodes",
					Severity:     "ok",
					Title:        fmt.Sprintf("Node %s fits scheduled requests and limits", nodeName),
					Summary:      "Combined pod requests and limits stay within node allocatable resources.",
					ResourceKind: "Node",
					ResourceName: nodeName,
					Node:         nodeName,
					Details: []string{
						fmt.Sprintf("CPU requests: %dm / %dm allocatable", usage.RequestCPUMilli, allocCPU),
						fmt.Sprintf("CPU limits: %dm / %dm allocatable", usage.LimitCPUMilli, allocCPU),
						fmt.Sprintf("Memory requests: %s / %s allocatable", formatBytesIEC(usage.RequestMemBytes), formatBytesIEC(allocMem)),
						fmt.Sprintf("Memory limits: %s / %s allocatable", formatBytesIEC(usage.LimitMemBytes), formatBytesIEC(allocMem)),
					},
				})
				continue
			}
			sort.Strings(usage.Pods)
			alerts = append(alerts, InsightAlert{
				Key:          fmt.Sprintf("node-overcommit:%s", nodeName),
				CheckType:    "node-capacity",
				CheckLabel:   "Node Capacity",
				Status:       "alert",
				Category:     "Nodes",
				Severity:     "critical",
				Title:        fmt.Sprintf("Node %s is overcommitted", nodeName),
				Summary:      "Combined pod requests or limits exceed node allocatable resources.",
				ResourceKind: "Node",
				ResourceName: nodeName,
				Node:         nodeName,
				Details:      append(details, fmt.Sprintf("Scheduled pods: %s", strings.Join(usage.Pods, ", "))),
			})
		}
	}

	sort.Slice(alerts, func(i, j int) bool {
		if alerts[i].Category != alerts[j].Category {
			return alerts[i].Category < alerts[j].Category
		}
		if alerts[i].CheckLabel != alerts[j].CheckLabel {
			return alerts[i].CheckLabel < alerts[j].CheckLabel
		}
		if alerts[i].Status != alerts[j].Status {
			return alerts[i].Status > alerts[j].Status
		}
		if alerts[i].Severity != alerts[j].Severity {
			return alerts[i].Severity > alerts[j].Severity
		}
		return alerts[i].Title < alerts[j].Title
	})
	return alerts, nil
}

func firstString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func normalizeInsightCategory(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "all":
		return ""
	case "node", "nodes":
		return "nodes"
	case "workload", "workloads", "pod", "pods":
		return "workloads"
	case "gpu", "gpus", "accelerator", "accelerators":
		return "gpu"
	case "network", "networking", "service", "services", "ingress", "ingresses":
		return "networking"
	case "storage", "pvc", "pv", "volume", "volumes":
		return "storage"
	case "security", "secure":
		return "security"
	case "configuration", "config", "references", "reference":
		return "configuration"
	default:
		return unsupportedInsightCategory
	}
}

func nodeConditionProblems(node *corev1.Node) []string {
	if node == nil {
		return nil
	}
	problems := make([]string, 0)
	for _, condition := range node.Status.Conditions {
		switch condition.Type {
		case corev1.NodeReady:
			if condition.Status != corev1.ConditionTrue {
				problems = append(problems, fmt.Sprintf("Ready is %s: %s", condition.Status, firstInsightValue(condition.Message, condition.Reason)))
			}
		case corev1.NodeMemoryPressure, corev1.NodeDiskPressure, corev1.NodePIDPressure, corev1.NodeNetworkUnavailable:
			if condition.Status == corev1.ConditionTrue {
				problems = append(problems, fmt.Sprintf("%s is active: %s", condition.Type, firstInsightValue(condition.Message, condition.Reason)))
			}
		}
	}
	return problems
}

func firstInsightValue(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return "-"
}

func podActive(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}
	return pod.Status.Phase != corev1.PodSucceeded && pod.Status.Phase != corev1.PodFailed
}

func podReady(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

func podUnreadySince(pod *corev1.Pod) time.Time {
	if pod == nil {
		return time.Now()
	}
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && !condition.LastTransitionTime.IsZero() {
			return condition.LastTransitionTime.Time
		}
	}
	return pod.CreationTimestamp.Time
}

func podWaitingReasonDetails(pod *corev1.Pod, allowed map[string]struct{}) []string {
	if pod == nil {
		return nil
	}
	out := make([]string, 0)
	add := func(prefix string, status corev1.ContainerStatus) {
		if status.State.Waiting == nil {
			return
		}
		reason := strings.TrimSpace(status.State.Waiting.Reason)
		if reason == "" {
			return
		}
		if allowed != nil {
			if _, ok := allowed[reason]; !ok {
				return
			}
		}
		message := strings.TrimSpace(status.State.Waiting.Message)
		if message != "" {
			out = append(out, fmt.Sprintf("%s %s waiting: %s - %s", prefix, status.Name, reason, message))
			return
		}
		out = append(out, fmt.Sprintf("%s %s waiting: %s", prefix, status.Name, reason))
	}
	for _, status := range pod.Status.InitContainerStatuses {
		add("init container", status)
	}
	for _, status := range pod.Status.ContainerStatuses {
		add("container", status)
	}
	sort.Strings(out)
	return out
}

func podOOMKilledDetails(pod *corev1.Pod) []string {
	if pod == nil {
		return nil
	}
	out := make([]string, 0)
	addTerminated := func(prefix string, name string, terminated *corev1.ContainerStateTerminated) {
		if terminated == nil || terminated.Reason != "OOMKilled" {
			return
		}
		out = append(out, fmt.Sprintf("%s %s was OOMKilled with exit code %d", prefix, name, terminated.ExitCode))
	}
	for _, status := range pod.Status.InitContainerStatuses {
		addTerminated("init container", status.Name, status.LastTerminationState.Terminated)
		addTerminated("init container", status.Name, status.State.Terminated)
	}
	for _, status := range pod.Status.ContainerStatuses {
		addTerminated("container", status.Name, status.LastTerminationState.Terminated)
		addTerminated("container", status.Name, status.State.Terminated)
	}
	sort.Strings(out)
	return out
}

func podRestartDetails(pod *corev1.Pod) (int32, []string) {
	if pod == nil {
		return 0, nil
	}
	var total int32
	details := make([]string, 0)
	add := func(prefix string, status corev1.ContainerStatus) {
		if status.RestartCount <= 0 {
			return
		}
		total += status.RestartCount
		details = append(details, fmt.Sprintf("%s %s restarts: %d", prefix, status.Name, status.RestartCount))
	}
	for _, status := range pod.Status.InitContainerStatuses {
		add("init container", status)
	}
	for _, status := range pod.Status.ContainerStatuses {
		add("container", status)
	}
	sort.Strings(details)
	return total, details
}

func podMissingResourceRequestDetails(pod *corev1.Pod) []string {
	if pod == nil {
		return nil
	}
	out := make([]string, 0)
	add := func(prefix string, container corev1.Container) {
		if container.Resources.Requests.Cpu().IsZero() {
			out = append(out, fmt.Sprintf("%s %s has no CPU request", prefix, container.Name))
		}
		if container.Resources.Requests.Memory().IsZero() {
			out = append(out, fmt.Sprintf("%s %s has no memory request", prefix, container.Name))
		}
	}
	for _, container := range pod.Spec.InitContainers {
		add("init container", container)
	}
	for _, container := range pod.Spec.Containers {
		add("container", container)
	}
	sort.Strings(out)
	return out
}

func podPrivilegeDetails(pod *corev1.Pod) []string {
	if pod == nil {
		return nil
	}
	out := make([]string, 0)
	if pod.Spec.HostNetwork {
		out = append(out, "pod uses hostNetwork")
	}
	if pod.Spec.HostPID {
		out = append(out, "pod uses hostPID")
	}
	if pod.Spec.HostIPC {
		out = append(out, "pod uses hostIPC")
	}
	add := func(prefix string, container corev1.Container) {
		if container.SecurityContext == nil {
			return
		}
		if container.SecurityContext.Privileged != nil && *container.SecurityContext.Privileged {
			out = append(out, fmt.Sprintf("%s %s is privileged", prefix, container.Name))
		}
		if container.SecurityContext.AllowPrivilegeEscalation != nil && *container.SecurityContext.AllowPrivilegeEscalation {
			out = append(out, fmt.Sprintf("%s %s allows privilege escalation", prefix, container.Name))
		}
	}
	for _, container := range pod.Spec.InitContainers {
		add("init container", container)
	}
	for _, container := range pod.Spec.Containers {
		add("container", container)
	}
	sort.Strings(out)
	return out
}

func podSensitiveLiteralEnvDetails(pod *corev1.Pod) []string {
	if pod == nil {
		return nil
	}
	out := make([]string, 0)
	add := func(prefix, containerName string, env []corev1.EnvVar) {
		for _, item := range env {
			if item.ValueFrom != nil || item.Value == "" || !sensitiveEnvName(item.Name) {
				continue
			}
			out = append(out, fmt.Sprintf("%s %s defines %s as a literal environment variable", prefix, containerName, item.Name))
		}
	}
	for _, container := range pod.Spec.InitContainers {
		add("init container", container.Name, container.Env)
	}
	for _, container := range pod.Spec.Containers {
		add("container", container.Name, container.Env)
	}
	for _, container := range pod.Spec.EphemeralContainers {
		add("ephemeral container", container.Name, container.Env)
	}
	sort.Strings(out)
	return out
}

func sensitiveEnvName(name string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(name))
	if normalized == "" {
		return false
	}
	sensitiveTokens := []string{
		"PASSWORD",
		"PASSWD",
		"SECRET",
		"TOKEN",
		"API_KEY",
		"PRIVATE_KEY",
		"ACCESS_KEY",
	}
	for _, token := range sensitiveTokens {
		if strings.Contains(normalized, token) {
			return true
		}
	}
	return false
}

func podReferenceProblems(pod *corev1.Pod, secretNames, configMapNames map[string]struct{}) []string {
	if pod == nil {
		return nil
	}
	out := make([]string, 0)
	addSecret := func(context, name string, optional bool) {
		name = strings.TrimSpace(name)
		if name == "" || optional {
			return
		}
		if _, ok := secretNames[name]; !ok {
			out = append(out, fmt.Sprintf("%s references missing Secret %s", context, name))
		}
	}
	addConfigMap := func(context, name string, optional bool) {
		name = strings.TrimSpace(name)
		if name == "" || optional {
			return
		}
		if _, ok := configMapNames[name]; !ok {
			out = append(out, fmt.Sprintf("%s references missing ConfigMap %s", context, name))
		}
	}
	for _, ref := range pod.Spec.ImagePullSecrets {
		addSecret("imagePullSecrets", ref.Name, false)
	}
	for _, volume := range pod.Spec.Volumes {
		if volume.Secret != nil {
			addSecret("volume "+volume.Name, volume.Secret.SecretName, boolValue(volume.Secret.Optional))
		}
		if volume.ConfigMap != nil {
			addConfigMap("volume "+volume.Name, volume.ConfigMap.Name, boolValue(volume.ConfigMap.Optional))
		}
		if volume.Projected != nil {
			for _, source := range volume.Projected.Sources {
				if source.Secret != nil {
					addSecret("projected volume "+volume.Name, source.Secret.Name, boolValue(source.Secret.Optional))
				}
				if source.ConfigMap != nil {
					addConfigMap("projected volume "+volume.Name, source.ConfigMap.Name, boolValue(source.ConfigMap.Optional))
				}
			}
		}
	}
	addContainerRefs := func(prefix string, name string, env []corev1.EnvVar, envFrom []corev1.EnvFromSource) {
		context := strings.TrimSpace(prefix + " " + name)
		for _, source := range envFrom {
			if source.SecretRef != nil {
				addSecret(context+" envFrom", source.SecretRef.Name, boolValue(source.SecretRef.Optional))
			}
			if source.ConfigMapRef != nil {
				addConfigMap(context+" envFrom", source.ConfigMapRef.Name, boolValue(source.ConfigMapRef.Optional))
			}
		}
		for _, item := range env {
			if item.ValueFrom == nil {
				continue
			}
			if item.ValueFrom.SecretKeyRef != nil {
				addSecret(context+" env "+item.Name, item.ValueFrom.SecretKeyRef.Name, boolValue(item.ValueFrom.SecretKeyRef.Optional))
			}
			if item.ValueFrom.ConfigMapKeyRef != nil {
				addConfigMap(context+" env "+item.Name, item.ValueFrom.ConfigMapKeyRef.Name, boolValue(item.ValueFrom.ConfigMapKeyRef.Optional))
			}
		}
	}
	for _, container := range pod.Spec.InitContainers {
		addContainerRefs("init container", container.Name, container.Env, container.EnvFrom)
	}
	for _, container := range pod.Spec.Containers {
		addContainerRefs("container", container.Name, container.Env, container.EnvFrom)
	}
	for _, container := range pod.Spec.EphemeralContainers {
		addContainerRefs("ephemeral container", container.Name, container.Env, container.EnvFrom)
	}
	sort.Strings(out)
	return out
}

func boolValue(value *bool) bool {
	return value != nil && *value
}

func podUnderRequestDetails(reqCPU, reqMem int64, usage usageValues) []string {
	out := make([]string, 0, 2)
	if reqCPU >= minUnderRequestCPUMilli && usage.cpuMilli*100 < reqCPU*underRequestCPUPercent {
		out = append(out, fmt.Sprintf("CPU used %dm is below %d%% of request %dm", usage.cpuMilli, underRequestCPUPercent, reqCPU))
	}
	if reqMem >= minUnderRequestMemoryBytes && usage.memoryBytes*100 < reqMem*underRequestMemoryPercent {
		out = append(out, fmt.Sprintf("Memory used %s is below %d%% of request %s", formatBytesIEC(usage.memoryBytes), underRequestMemoryPercent, formatBytesIEC(reqMem)))
	}
	return out
}

func namespaceGPUQuotaPresent(quotas []corev1.ResourceQuota) bool {
	gpuQuotaNames := []corev1.ResourceName{
		corev1.ResourceName("requests.nvidia.com/gpu"),
		corev1.ResourceName("limits.nvidia.com/gpu"),
		corev1.ResourceName("nvidia.com/gpu"),
	}
	for _, quota := range quotas {
		for _, resourceName := range gpuQuotaNames {
			if _, ok := quota.Spec.Hard[resourceName]; ok {
				return true
			}
		}
	}
	return false
}

func daemonSetReadinessInsight(ns string, daemonSet *appsv1.DaemonSet) InsightAlert {
	details := []string{
		fmt.Sprintf("Desired scheduled: %d", daemonSet.Status.DesiredNumberScheduled),
		fmt.Sprintf("Current scheduled: %d", daemonSet.Status.CurrentNumberScheduled),
		fmt.Sprintf("Ready: %d", daemonSet.Status.NumberReady),
		fmt.Sprintf("Available: %d", daemonSet.Status.NumberAvailable),
		fmt.Sprintf("Updated: %d", daemonSet.Status.UpdatedNumberScheduled),
	}
	if daemonSet.Status.DesiredNumberScheduled > daemonSet.Status.NumberReady {
		return InsightAlert{
			Key:          fmt.Sprintf("daemonset-not-ready:%s:%s", ns, daemonSet.Name),
			CheckType:    "daemonset-readiness",
			CheckLabel:   "DaemonSet Readiness",
			Status:       "alert",
			Category:     "Workloads",
			Severity:     "warning",
			Title:        fmt.Sprintf("DaemonSet %s/%s is not ready on all scheduled nodes", ns, daemonSet.Name),
			Summary:      "DaemonSet has fewer ready pods than desired scheduled pods.",
			Namespace:    ns,
			ResourceKind: "DaemonSet",
			ResourceName: daemonSet.Name,
			Details:      details,
		}
	}
	return InsightAlert{
		Key:          fmt.Sprintf("daemonset-not-ready:%s:%s", ns, daemonSet.Name),
		CheckType:    "daemonset-readiness",
		CheckLabel:   "DaemonSet Readiness",
		Status:       "ok",
		Category:     "Workloads",
		Severity:     "ok",
		Title:        fmt.Sprintf("DaemonSet %s/%s is ready", ns, daemonSet.Name),
		Summary:      "DaemonSet ready pod count matches desired scheduled pods.",
		Namespace:    ns,
		ResourceKind: "DaemonSet",
		ResourceName: daemonSet.Name,
		Details:      details,
	}
}

func eventDetails(events []corev1.Event, reasons ...string) []string {
	if len(events) == 0 {
		return nil
	}
	reasonSet := make(map[string]struct{}, len(reasons))
	for _, reason := range reasons {
		reasonSet[reason] = struct{}{}
	}
	out := make([]string, 0)
	for _, event := range events {
		if len(reasonSet) > 0 {
			if _, ok := reasonSet[event.Reason]; !ok {
				continue
			}
		}
		out = append(out, fmt.Sprintf("Event %s: %s", event.Reason, strings.TrimSpace(event.Message)))
	}
	sort.Strings(out)
	return limitedDetails(out, 6)
}

func limitedDetails(in []string, limit int) []string {
	if limit <= 0 || len(in) <= limit {
		return append([]string{}, in...)
	}
	out := append([]string{}, in[:limit]...)
	out = append(out, fmt.Sprintf("...and %d more", len(in)-limit))
	return out
}

func gpuAllocationPressureInsight(scope, name string, alloc, requested int64) InsightAlert {
	status := "ok"
	severity := "ok"
	ratio := 0.0
	if alloc > 0 {
		ratio = float64(requested) / float64(alloc)
		switch {
		case ratio >= gpuAllocationCriticalRatio:
			status = "alert"
			severity = "critical"
		case ratio >= gpuAllocationWarningRatio:
			status = "alert"
			severity = "warning"
		}
	}
	title := fmt.Sprintf("GPU allocation for %s %s is %.0f%%", scope, name, ratio*100)
	summary := "GPU requests are below allocation pressure thresholds."
	if status == "alert" {
		summary = "GPU requests are close to or above allocatable GPU capacity."
	}
	alert := InsightAlert{
		Key:        fmt.Sprintf("gpu-allocation-pressure:%s:%s", scope, name),
		CheckType:  "gpu-allocation-pressure",
		CheckLabel: "GPU Allocation Pressure",
		Status:     status,
		Category:   "GPU",
		Severity:   severity,
		Title:      title,
		Summary:    summary,
		Details: []string{
			fmt.Sprintf("Requested GPUs: %d", requested),
			fmt.Sprintf("Allocatable GPUs: %d", alloc),
			fmt.Sprintf("Allocated: %.1f%%", ratio*100),
		},
	}
	if scope == "node" {
		alert.ResourceKind = "Node"
		alert.ResourceName = name
		alert.Node = name
	}
	return alert
}

func ingressRouteKeys(ingress *networkingv1.Ingress) map[string]string {
	out := make(map[string]string)
	if ingress == nil {
		return out
	}
	className := "-"
	if ingress.Spec.IngressClassName != nil && strings.TrimSpace(*ingress.Spec.IngressClassName) != "" {
		className = strings.TrimSpace(*ingress.Spec.IngressClassName)
	}
	add := func(host, path string) {
		host = strings.TrimSpace(host)
		if host == "" {
			host = "*"
		}
		path = strings.TrimSpace(path)
		if path == "" {
			path = "/"
		}
		key := className + "|" + host + "|" + path
		out[key] = fmt.Sprintf("class=%s host=%s path=%s", className, host, path)
	}
	if ingress.Spec.DefaultBackend != nil {
		add("*", "/")
	}
	for _, rule := range ingress.Spec.Rules {
		if rule.HTTP == nil {
			continue
		}
		for _, path := range rule.HTTP.Paths {
			add(rule.Host, path.Path)
		}
	}
	return out
}
