package kube

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func (c *Client) ListNamespaces(ctx context.Context) ([]string, error) {
	list, err := c.core.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(list.Items))
	for _, ns := range list.Items {
		out = append(out, ns.Name)
	}
	sort.Strings(out)
	return out, nil
}

func (c *Client) ListWorkloads(ctx context.Context, ns string) ([]Workload, error) {
	out := make([]Workload, 0)

	var (
		wg       sync.WaitGroup
		deps     *appsv1.DeploymentList
		sts      *appsv1.StatefulSetList
		dss      *appsv1.DaemonSetList
		jobs     *batchv1.JobList
		cronJobs *batchv1.CronJobList
		depsErr  error
		stsErr   error
		dssErr   error
		jobsErr  error
		cjErr    error
	)
	wg.Add(5)
	go func() {
		defer wg.Done()
		deps, depsErr = c.core.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{})
	}()
	go func() {
		defer wg.Done()
		sts, stsErr = c.core.AppsV1().StatefulSets(ns).List(ctx, metav1.ListOptions{})
	}()
	go func() {
		defer wg.Done()
		dss, dssErr = c.core.AppsV1().DaemonSets(ns).List(ctx, metav1.ListOptions{})
	}()
	go func() {
		defer wg.Done()
		jobs, jobsErr = c.core.BatchV1().Jobs(ns).List(ctx, metav1.ListOptions{})
	}()
	go func() {
		defer wg.Done()
		cronJobs, cjErr = c.core.BatchV1().CronJobs(ns).List(ctx, metav1.ListOptions{})
	}()
	wg.Wait()
	if depsErr != nil {
		return nil, depsErr
	}
	if stsErr != nil {
		return nil, stsErr
	}
	if dssErr != nil {
		return nil, dssErr
	}
	if jobsErr != nil {
		return nil, jobsErr
	}
	if cjErr != nil {
		return nil, cjErr
	}

	for _, d := range deps.Items {
		targetReplicas := int32(1)
		if d.Spec.Replicas != nil {
			targetReplicas = *d.Spec.Replicas
		}
		out = append(out, Workload{
			Kind:      "Deployment",
			Namespace: ns,
			Name:      d.Name,
			Ready:     fmt.Sprintf("%d/%d", d.Status.ReadyReplicas, targetReplicas),
			Age:       age(d.CreationTimestamp.Time),
		})
	}
	for _, s := range sts.Items {
		replicas := int32(0)
		if s.Spec.Replicas != nil {
			replicas = *s.Spec.Replicas
		}
		out = append(out, Workload{
			Kind:      "StatefulSet",
			Namespace: ns,
			Name:      s.Name,
			Ready:     fmt.Sprintf("%d/%d", s.Status.ReadyReplicas, replicas),
			Age:       age(s.CreationTimestamp.Time),
		})
	}
	for _, d := range dss.Items {
		out = append(out, Workload{
			Kind:      "DaemonSet",
			Namespace: ns,
			Name:      d.Name,
			Ready:     fmt.Sprintf("%d/%d", d.Status.NumberReady, d.Status.DesiredNumberScheduled),
			Age:       age(d.CreationTimestamp.Time),
		})
	}
	for _, job := range jobs.Items {
		desiredCompletions := int32(1)
		if job.Spec.Completions != nil {
			desiredCompletions = *job.Spec.Completions
		}
		out = append(out, Workload{
			Kind:      "Job",
			Namespace: ns,
			Name:      job.Name,
			Ready:     fmt.Sprintf("%d/%d", job.Status.Succeeded, desiredCompletions),
			Age:       age(job.CreationTimestamp.Time),
		})
	}
	for _, cronJob := range cronJobs.Items {
		out = append(out, Workload{
			Kind:      "CronJob",
			Namespace: ns,
			Name:      cronJob.Name,
			Ready:     fmt.Sprintf("%d active", len(cronJob.Status.Active)),
			Age:       age(cronJob.CreationTimestamp.Time),
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func (c *Client) ListPods(ctx context.Context, ns string, includeMetrics bool) ([]PodInfo, error) {
	pods, err := c.core.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	usageByPod := make(map[string]usageValues)
	metricsAvailable := false
	dcgmMetrics := dcgmMetricsSnapshot{
		nodeUsage: map[string]gpuUsageValues{},
		podUsage:  map[string]gpuUsageValues{},
	}
	if includeMetrics {
		usageByPod, metricsAvailable = c.collectPodUsageMetrics(ctx, ns)
		dcgmMetrics = c.collectDCGMMetrics(ctx)
	}

	out := make([]PodInfo, 0, len(pods.Items))
	for _, p := range pods.Items {
		var (
			ready    int
			restarts int32
		)
		for _, cs := range p.Status.ContainerStatuses {
			if cs.Ready {
				ready++
			}
			restarts += cs.RestartCount
		}
		reqCPU, limCPU, reqMem, limMem := podResourceTotals(&p)
		gpuRequestCount := podGPURequestCount(&p)
		gpuUsage := dcgmMetrics.podUsage[ns+"/"+p.Name]
		usage := usageByPod[p.Name]
		cpuDisplay := "-"
		memoryDisplay := "-"
		gpuDisplay := formatGPUUsage(gpuUsage, gpuUsage.deviceCount)
		if includeMetrics {
			cpuDisplay = formatMilliUsageUnknown(metricsAvailable, usage.cpuMilli, limCPU)
			memoryDisplay = formatByteUsageUnknown(metricsAvailable, usage.memoryBytes, limMem)
		}
		out = append(out, PodInfo{
			Namespace:           ns,
			Name:                p.Name,
			Phase:               string(p.Status.Phase),
			Ready:               fmt.Sprintf("%d/%d", ready, len(p.Status.ContainerStatuses)),
			Restarts:            restarts,
			Age:                 age(p.CreationTimestamp.Time),
			Node:                p.Spec.NodeName,
			Containers:          podContainerNames(&p),
			MetricsAvailable:    metricsAvailable,
			CPU:                 cpuDisplay,
			CPUUsedMilli:        usage.cpuMilli,
			CPURequestMilli:     reqCPU,
			CPULimitMilli:       limCPU,
			CPUTotalMilli:       limCPU,
			Memory:              memoryDisplay,
			MemoryUsedBytes:     usage.memoryBytes,
			MemoryRequestBytes:  reqMem,
			MemoryLimitBytes:    limMem,
			MemoryTotalBytes:    limMem,
			GPU:                 gpuDisplay,
			GPUMetricsAvailable: dcgmMetrics.available && (gpuUsage.hasUtil || gpuUsage.hasMemory),
			GPUUsedPercent:      gpuUsage.utilPercent,
			GPUMemoryUsedBytes:  gpuUsage.memoryUsedBytes,
			GPURequestCount:     gpuRequestCount,
			GPUDeviceCount:      gpuUsage.deviceCount,
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func podContainerNames(pod *corev1.Pod) []string {
	out := make([]string, 0, len(pod.Spec.Containers))
	for _, container := range pod.Spec.Containers {
		if container.Name == "" {
			continue
		}
		out = append(out, container.Name)
	}
	sort.Strings(out)
	return out
}

func (c *Client) ListCRDs(ctx context.Context) ([]CRDInfo, error) {
	items, err := c.dyn.Resource(schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	}).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	out := make([]CRDInfo, 0, len(items.Items))
	for _, item := range items.Items {
		group, _, _ := unstructured.NestedString(item.Object, "spec", "group")
		kind, _, _ := unstructured.NestedString(item.Object, "spec", "names", "kind")
		resource, _, _ := unstructured.NestedString(item.Object, "spec", "names", "plural")
		scope, _, _ := unstructured.NestedString(item.Object, "spec", "scope")
		versionsRaw, found, _ := unstructured.NestedSlice(item.Object, "spec", "versions")
		versions := make([]string, 0)
		preferredVersion := ""
		if found {
			for _, raw := range versionsRaw {
				version, ok := raw.(map[string]any)
				if !ok {
					continue
				}
				name, _ := version["name"].(string)
				if name == "" {
					continue
				}
				served, _ := version["served"].(bool)
				storage, _ := version["storage"].(bool)
				if storage && served {
					preferredVersion = name
				} else if served && preferredVersion == "" {
					preferredVersion = name
				}
				switch {
				case storage:
					versions = append(versions, name+" (storage)")
				case served:
					versions = append(versions, name+" (served)")
				default:
					versions = append(versions, name)
				}
			}
		}
		out = append(out, CRDInfo{
			Name:     item.GetName(),
			Group:    group,
			Kind:     kind,
			Resource: resource,
			Version:  preferredVersion,
			Scope:    scope,
			Versions: strings.Join(versions, ", "),
			Age:      age(item.GetCreationTimestamp().Time),
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (c *Client) ResolveCustomResourceDefinition(ctx context.Context, name string) (CustomResourceDefinition, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return CustomResourceDefinition{}, fmt.Errorf("CRD name is required")
	}
	item, err := c.dyn.Resource(schema.GroupVersionResource{
		Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions",
	}).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return CustomResourceDefinition{}, err
	}
	group, _, _ := unstructured.NestedString(item.Object, "spec", "group")
	kind, _, _ := unstructured.NestedString(item.Object, "spec", "names", "kind")
	resource, _, _ := unstructured.NestedString(item.Object, "spec", "names", "plural")
	scope, _, _ := unstructured.NestedString(item.Object, "spec", "scope")
	versions, _, _ := unstructured.NestedSlice(item.Object, "spec", "versions")
	version := ""
	for _, raw := range versions {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		name, _ := entry["name"].(string)
		served, _ := entry["served"].(bool)
		storage, _ := entry["storage"].(bool)
		if served && storage {
			version = name
			break
		}
		if served && version == "" {
			version = name
		}
	}
	if group == "" || version == "" || resource == "" || kind == "" {
		return CustomResourceDefinition{}, fmt.Errorf("CRD %s has no usable served resource", name)
	}
	return CustomResourceDefinition{
		Name: name, Group: group, Version: version, Resource: resource, Kind: kind,
		Namespaced: strings.EqualFold(scope, "Namespaced"),
	}, nil
}

func (c *Client) ListCustomResources(ctx context.Context, definition CustomResourceDefinition, namespaces []string) ([]CustomResourceInfo, error) {
	gvr := schema.GroupVersionResource{Group: definition.Group, Version: definition.Version, Resource: definition.Resource}
	items := make([]unstructured.Unstructured, 0)
	if definition.Namespaced {
		for _, namespace := range namespaces {
			list, err := c.dyn.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
			if err != nil {
				return nil, err
			}
			items = append(items, list.Items...)
		}
	} else {
		list, err := c.dyn.Resource(gvr).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		items = append(items, list.Items...)
	}
	out := make([]CustomResourceInfo, 0, len(items))
	for _, item := range items {
		out = append(out, CustomResourceInfo{
			Namespace: item.GetNamespace(), Name: item.GetName(), APIVersion: item.GetAPIVersion(),
			Kind: item.GetKind(), Age: age(item.GetCreationTimestamp().Time),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace != out[j].Namespace {
			return out[i].Namespace < out[j].Namespace
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func (c *Client) ListNodes(ctx context.Context) ([]NodeInfo, error) {
	nodes, err := c.core.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	pods, err := c.core.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	usageByNode, metricsAvailable := c.collectNodeUsageMetrics(ctx)
	dcgmMetrics := c.collectDCGMMetrics(ctx)
	podCountByNode := make(map[string]int64, len(nodes.Items))
	for _, pod := range pods.Items {
		if strings.TrimSpace(pod.Spec.NodeName) == "" {
			continue
		}
		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			continue
		}
		podCountByNode[pod.Spec.NodeName]++
	}

	out := make([]NodeInfo, 0, len(nodes.Items))
	for _, n := range nodes.Items {
		status := "Unknown"
		for _, cond := range n.Status.Conditions {
			if cond.Type == corev1.NodeReady {
				if cond.Status == corev1.ConditionTrue {
					status = "Ready"
				} else {
					status = "NotReady"
				}
				break
			}
		}

		roles := make([]string, 0)
		for k := range n.Labels {
			if strings.HasPrefix(k, "node-role.kubernetes.io/") {
				role := strings.TrimPrefix(k, "node-role.kubernetes.io/")
				if role == "" {
					role = "control-plane"
				}
				roles = append(roles, role)
			}
		}
		sort.Strings(roles)
		if len(roles) == 0 {
			roles = []string{"worker"}
		}

		cpuTotalMilli := n.Status.Allocatable.Cpu().MilliValue()
		memoryTotalBytes := n.Status.Allocatable.Memory().Value()
		gpuAlloc := n.Status.Allocatable[corev1.ResourceName("nvidia.com/gpu")]
		gpuTotal := gpuAlloc.Value()
		maxPodCount := n.Status.Allocatable.Pods().Value()
		podCount := podCountByNode[n.Name]
		usage := usageByNode[n.Name]
		gpuUsage := dcgmMetrics.nodeUsage[n.Name]
		cpuDisplay := "-"
		if cpuTotalMilli > 0 {
			cpuDisplay = fmt.Sprintf("%dm / %dm", usage.cpuMilli, cpuTotalMilli)
		}
		memoryDisplay := "-"
		if memoryTotalBytes > 0 {
			memoryDisplay = fmt.Sprintf("%s / %s", formatBytesIEC(usage.memoryBytes), formatBytesIEC(memoryTotalBytes))
		}
		podDisplay := "-"
		if maxPodCount > 0 {
			podDisplay = fmt.Sprintf("%d / %d", podCount, maxPodCount)
		}
		gpuDisplay := formatGPUUsage(gpuUsage, gpuTotal)

		out = append(out, NodeInfo{
			Name:                n.Name,
			Status:              status,
			Roles:               strings.Join(roles, ","),
			Age:                 age(n.CreationTimestamp.Time),
			Labels:              n.Labels,
			Pods:                podDisplay,
			PodCount:            podCount,
			MaxPodCount:         maxPodCount,
			MetricsAvailable:    metricsAvailable,
			CPU:                 cpuDisplay,
			CPUUsedMilli:        usage.cpuMilli,
			CPUTotalMilli:       cpuTotalMilli,
			Memory:              memoryDisplay,
			MemoryUsedBytes:     usage.memoryBytes,
			MemoryTotalBytes:    memoryTotalBytes,
			HasGPU:              gpuTotal > 0,
			GPUCount:            gpuTotal,
			GPU:                 gpuDisplay,
			GPUMetricsAvailable: dcgmMetrics.available && (gpuUsage.hasUtil || gpuUsage.hasMemory),
			GPUUsedPercent:      gpuUsage.utilPercent,
			GPUMemoryUsedBytes:  gpuUsage.memoryUsedBytes,
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (c *Client) ListIngresses(ctx context.Context, ns string) ([]IngressInfo, error) {
	items, err := c.core.NetworkingV1().Ingresses(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	out := make([]IngressInfo, 0, len(items.Items))
	for _, item := range items.Items {
		hosts := make([]string, 0, len(item.Spec.Rules))
		for _, r := range item.Spec.Rules {
			if r.Host != "" {
				hosts = append(hosts, r.Host)
			}
		}
		sort.Strings(hosts)
		if len(hosts) == 0 {
			hosts = []string{"*"}
		}

		addresses := make([]string, 0, len(item.Status.LoadBalancer.Ingress))
		for _, addr := range item.Status.LoadBalancer.Ingress {
			if addr.Hostname != "" {
				addresses = append(addresses, addr.Hostname)
				continue
			}
			if addr.IP != "" {
				addresses = append(addresses, addr.IP)
			}
		}
		if len(addresses) == 0 {
			addresses = []string{"-"}
		}

		class := "-"
		if item.Spec.IngressClassName != nil && *item.Spec.IngressClassName != "" {
			class = *item.Spec.IngressClassName
		}

		out = append(out, IngressInfo{
			Namespace: ns,
			Name:      item.Name,
			Class:     class,
			Hosts:     strings.Join(hosts, ","),
			Address:   strings.Join(addresses, ","),
			Age:       age(item.CreationTimestamp.Time),
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (c *Client) ListSecrets(ctx context.Context, ns string) ([]SecretInfo, error) {
	items, err := c.core.CoreV1().Secrets(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]SecretInfo, 0, len(items.Items))
	for _, item := range items.Items {
		out = append(out, SecretInfo{
			Namespace: ns,
			Name:      item.Name,
			Type:      string(item.Type),
			DataKeys:  len(item.Data),
			Age:       age(item.CreationTimestamp.Time),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (c *Client) ListConfigMaps(ctx context.Context, ns string) ([]ConfigMapInfo, error) {
	items, err := c.core.CoreV1().ConfigMaps(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]ConfigMapInfo, 0, len(items.Items))
	for _, item := range items.Items {
		out = append(out, ConfigMapInfo{
			Namespace: ns,
			Name:      item.Name,
			DataKeys:  len(item.Data),
			Age:       age(item.CreationTimestamp.Time),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (c *Client) ListServices(ctx context.Context, ns string) ([]ServiceInfo, error) {
	items, err := c.core.CoreV1().Services(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]ServiceInfo, 0, len(items.Items))
	for _, item := range items.Items {
		ports := make([]string, 0, len(item.Spec.Ports))
		for _, p := range item.Spec.Ports {
			ports = append(ports, fmt.Sprintf("%d/%s", p.Port, strings.ToLower(string(p.Protocol))))
		}
		out = append(out, ServiceInfo{
			Namespace: ns,
			Name:      item.Name,
			Type:      string(item.Spec.Type),
			ClusterIP: item.Spec.ClusterIP,
			Ports:     strings.Join(ports, ","),
			Age:       age(item.CreationTimestamp.Time),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (c *Client) ListPVCs(ctx context.Context, namespaces []string) ([]PVCInfo, error) {
	nsList := uniqueStrings(namespaces)
	usageByPVC, statsAvailable, _ := c.collectPVCVolumeStats(ctx)

	out := make([]PVCInfo, 0)
	for _, ns := range nsList {
		items, err := c.core.CoreV1().PersistentVolumeClaims(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		for _, item := range items.Items {
			capacity := "-"
			if q, ok := item.Status.Capacity[corev1.ResourceStorage]; ok {
				capacity = q.String()
			}
			storageClass := "-"
			if item.Spec.StorageClassName != nil && *item.Spec.StorageClassName != "" {
				storageClass = *item.Spec.StorageClassName
			}
			usage := usageByPVC[item.Namespace+"/"+item.Name]
			out = append(out, PVCInfo{
				Namespace:        ns,
				Name:             item.Name,
				Status:           string(item.Status.Phase),
				Volume:           item.Spec.VolumeName,
				Capacity:         capacity,
				StorageClass:     storageClass,
				Usage:            formatByteUsage(usage.UsedBytes, usage.CapacityBytes),
				MetricsAvailable: statsAvailable,
				UsedBytes:        usage.UsedBytes,
				CapacityBytes:    usage.CapacityBytes,
				Age:              age(item.CreationTimestamp.Time),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace != out[j].Namespace {
			return out[i].Namespace < out[j].Namespace
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func (c *Client) ListPVs(ctx context.Context) ([]PVInfo, error) {
	items, err := c.core.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	usageByPVC, statsAvailable, _ := c.collectPVCVolumeStats(ctx)

	out := make([]PVInfo, 0, len(items.Items))
	for _, item := range items.Items {
		capacity := "-"
		if q, ok := item.Spec.Capacity[corev1.ResourceStorage]; ok {
			capacity = q.String()
		}

		claim := "-"
		usage := pvcVolumeUsage{}
		if item.Spec.ClaimRef != nil {
			claim = fmt.Sprintf("%s/%s", item.Spec.ClaimRef.Namespace, item.Spec.ClaimRef.Name)
			usage = usageByPVC[item.Spec.ClaimRef.Namespace+"/"+item.Spec.ClaimRef.Name]
		}

		out = append(out, PVInfo{
			Name:             item.Name,
			Status:           string(item.Status.Phase),
			Capacity:         capacity,
			Claim:            claim,
			StorageClass:     item.Spec.StorageClassName,
			Usage:            formatByteUsage(usage.UsedBytes, usage.CapacityBytes),
			MetricsAvailable: statsAvailable,
			UsedBytes:        usage.UsedBytes,
			CapacityBytes:    usage.CapacityBytes,
			Age:              age(item.CreationTimestamp.Time),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (c *Client) collectPVCVolumeStats(ctx context.Context) (map[string]pvcVolumeUsage, bool, error) {
	nodes, err := c.core.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, false, err
	}

	usageByPVC := make(map[string]pvcVolumeUsage)
	statsAvailable := false
	restClient := c.core.CoreV1().RESTClient()
	for _, node := range nodes.Items {
		path := fmt.Sprintf("/api/v1/nodes/%s/proxy/stats/summary", node.Name)
		raw, err := restClient.Get().AbsPath(path).DoRaw(ctx)
		if err != nil {
			continue
		}
		var summary summaryStats
		if err := json.Unmarshal(raw, &summary); err != nil {
			continue
		}
		statsAvailable = true
		for _, pod := range summary.Pods {
			for _, volume := range pod.Volumes {
				if volume.PVCRef == nil {
					continue
				}
				namespace := volume.PVCRef.Namespace
				if namespace == "" {
					namespace = pod.PodRef.Namespace
				}
				if namespace == "" || volume.PVCRef.Name == "" {
					continue
				}
				key := namespace + "/" + volume.PVCRef.Name
				current := usageByPVC[key]
				usedBytes := volume.UsedBytes
				capacityBytes := volume.CapacityBytes
				availableBytes := volume.AvailableBytes
				if volume.FS != nil {
					if usedBytes == nil {
						usedBytes = volume.FS.UsedBytes
					}
					if capacityBytes == nil {
						capacityBytes = volume.FS.CapacityBytes
					}
					if availableBytes == nil {
						availableBytes = volume.FS.AvailableBytes
					}
				}
				if usedBytes != nil && int64(*usedBytes) > current.UsedBytes {
					current.UsedBytes = int64(*usedBytes)
				}
				if capacityBytes != nil && int64(*capacityBytes) > current.CapacityBytes {
					current.CapacityBytes = int64(*capacityBytes)
				}
				if availableBytes != nil && int64(*availableBytes) > current.AvailableBytes {
					current.AvailableBytes = int64(*availableBytes)
				}
				usageByPVC[key] = current
			}
		}
	}

	return usageByPVC, statsAvailable, nil
}

func (c *Client) ListStorageClasses(ctx context.Context) ([]StorageClassInfo, error) {
	items, err := c.core.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	out := make([]StorageClassInfo, 0, len(items.Items))
	for _, item := range items.Items {
		reclaim := "-"
		if item.ReclaimPolicy != nil {
			reclaim = string(*item.ReclaimPolicy)
		}
		mode := "-"
		if item.VolumeBindingMode != nil {
			mode = string(*item.VolumeBindingMode)
		}
		defaultClass := item.Annotations["storageclass.kubernetes.io/is-default-class"] == "true" ||
			item.Annotations["storageclass.beta.kubernetes.io/is-default-class"] == "true"

		out = append(out, StorageClassInfo{
			Name:              item.Name,
			Provisioner:       item.Provisioner,
			ReclaimPolicy:     reclaim,
			VolumeBindingMode: mode,
			DefaultClass:      defaultClass,
			Age:               age(item.CreationTimestamp.Time),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
