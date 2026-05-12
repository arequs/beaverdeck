package kube

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	includeNetworking := includeAll || insightCategory == "networking"
	includeStorage := includeAll || insightCategory == "storage"
	includeNodePodCapacity := includeAll

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

	nodeAllocCPU := map[string]int64{}
	nodeAllocMem := map[string]int64{}
	usageByNode := map[string]*nodeUsage{}
	gpuNodes := make([]string, 0)
	nodesLoaded := false
	var nodes corev1.NodeList
	if includeNodes || includeNodePodCapacity {
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
			usageByNode[node.Name] = &nodeUsage{}
			gpuAlloc := node.Status.Allocatable[corev1.ResourceName("nvidia.com/gpu")]
			if gpuAlloc.Value() > 0 {
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
		resourceMetrics = c.resourceMetricsStatus(ctx)
	}
	dcgmMetrics := dcgmMetricsSnapshot{}
	if includeNodes && len(gpuNodes) > 0 {
		dcgmMetrics = c.collectDCGMMetrics(ctx)
	}
	allPodUsageByKey := map[string]usageValues{}
	podMetricsAvailable := false
	if includeWorkloads {
		allPodUsageByKey, podMetricsAvailable = c.collectAllPodUsageMetrics(ctx)
	}

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
	if includeNodes && len(gpuNodes) > 0 && !dcgmMetrics.available {
		details := make([]string, 0, len(gpuNodes)+2)
		details = append(details, fmt.Sprintf("GPU nodes detected: %d", len(gpuNodes)))
		for _, nodeName := range gpuNodes {
			details = append(details, fmt.Sprintf("Node: %s", nodeName))
		}
		details = append(details, "dcgm-exporter was not discovered or its /metrics endpoint could not be scraped.")
		alerts = append(alerts, InsightAlert{
			Key:        "cluster-dcgm-exporter",
			CheckType:  "gpu-metrics",
			CheckLabel: "GPU Metrics",
			Status:     "alert",
			Category:   "Observability",
			Severity:   "warning",
			Title:      "dcgm-exporter is unavailable on GPU nodes",
			Summary:    "GPU-capable nodes were detected, but BeaverDeck could not collect GPU metrics from dcgm-exporter.",
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
						Category:     "Capacity",
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
						Category:     "Capacity",
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
						Category:     "Capacity",
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
						Category:     "Capacity",
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

		if includeWorkloads || includeNodePodCapacity {
			pods, err := c.core.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
			if err != nil {
				return nil, err
			}
			for _, pod := range pods.Items {
				if includeNodePodCapacity && pod.Spec.NodeName != "" && pod.Status.Phase != corev1.PodSucceeded && pod.Status.Phase != corev1.PodFailed {
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

				if !includeWorkloads {
					continue
				}

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

				if strings.EqualFold(string(pod.Status.Phase), string(corev1.PodRunning)) && podMetricsAvailable {
					usage := allPodUsageByKey[ns+"/"+pod.Name]
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
							Category:     "Capacity",
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
							Category:     "Capacity",
							Severity:     "ok",
							Title:        fmt.Sprintf("Pod %s/%s stays within requests", ns, pod.Name),
							Summary:      "Current CPU and memory usage stay within the requested resources.",
							Namespace:    ns,
							ResourceKind: "Pod",
							ResourceName: pod.Name,
						})
					}
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

			secrets, err := c.core.CoreV1().Secrets(ns).List(ctx, metav1.ListOptions{})
			if err != nil {
				return nil, err
			}
			secretByName := make(map[string]corev1.Secret, len(secrets.Items))
			for _, secret := range secrets.Items {
				secretByName[secret.Name] = secret
			}

			ingresses, err := c.core.NetworkingV1().Ingresses(ns).List(ctx, metav1.ListOptions{})
			if err != nil {
				return nil, err
			}
			for _, ingress := range ingresses.Items {
				tlsProblems := make([]string, 0)
				for _, tls := range ingress.Spec.TLS {
					if strings.TrimSpace(tls.SecretName) == "" {
						tlsProblems = append(tlsProblems, "TLS entry has no secretName")
						continue
					}
					secret, ok := secretByName[tls.SecretName]
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
					Category:     "Capacity",
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
				Category:     "Capacity",
				Severity:     "ok",
				Title:        fmt.Sprintf("PV %s was released recently", pv.Name),
				Summary:      "Released PersistentVolume is still within the 7-day cleanup window.",
				ResourceKind: "PersistentVolume",
				ResourceName: pv.Name,
				Details:      details,
			})
		}
	}

	if includeNodePodCapacity {
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
			if len(details) == 0 {
				alerts = append(alerts, InsightAlert{
					Key:          fmt.Sprintf("node-overcommit:%s", nodeName),
					CheckType:    "node-capacity",
					CheckLabel:   "Node Capacity",
					Status:       "ok",
					Category:     "Capacity",
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
				Category:     "Capacity",
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
	case "network", "networking", "service", "services", "ingress", "ingresses":
		return "networking"
	case "storage", "pvc", "pv", "volume", "volumes":
		return "storage"
	default:
		return ""
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
