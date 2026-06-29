package kube

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
)

func (c *Client) collectPodUsageMetrics(ctx context.Context, ns string) (map[string]usageValues, bool) {
	usageByPod, metricsAvailable := c.collectPodUsageMetricsFromMetricsAPI(ctx, ns)
	if metricsAvailable {
		return usageByPod, true
	}
	allUsage, directAvailable := c.collectAllPodUsageMetricsFromKubelet(ctx)
	filtered := make(map[string]usageValues)
	prefix := ns + "/"
	for key, usage := range allUsage {
		if strings.HasPrefix(key, prefix) {
			filtered[strings.TrimPrefix(key, prefix)] = usage
		}
	}
	return filtered, directAvailable
}

func (c *Client) collectPodUsageMetricsForNamespaces(ctx context.Context, namespaces []string, nodes []corev1.Node) (map[string]usageValues, bool) {
	nsList := uniqueStrings(namespaces)
	if len(nsList) == 0 {
		return map[string]usageValues{}, false
	}
	usageByPod := make(map[string]usageValues)
	metricsAvailable := true
	for _, ns := range nsList {
		namespaceUsage, ok := c.collectPodUsageMetricsFromMetricsAPI(ctx, ns)
		if !ok {
			metricsAvailable = false
			break
		}
		for podName, usage := range namespaceUsage {
			usageByPod[ns+"/"+podName] = usage
		}
	}
	if metricsAvailable {
		return usageByPod, true
	}
	if c.rest == nil {
		return map[string]usageValues{}, false
	}
	if len(nodes) == 0 {
		return c.collectAllPodUsageMetricsFromKubelet(ctx)
	}
	allUsage, directAvailable := c.collectAllPodUsageMetricsFromKubeletForNodes(ctx, nodes)
	return filterUsageByNamespaces(allUsage, nsList), directAvailable
}

func filterUsageByNamespaces(usageByPod map[string]usageValues, namespaces []string) map[string]usageValues {
	nsSet := make(map[string]struct{}, len(namespaces))
	for _, ns := range namespaces {
		nsSet[ns] = struct{}{}
	}
	filtered := make(map[string]usageValues)
	for key, usage := range usageByPod {
		ns, _, found := strings.Cut(key, "/")
		if !found {
			continue
		}
		if _, ok := nsSet[ns]; ok {
			filtered[key] = usage
		}
	}
	return filtered
}

func (c *Client) collectPodUsageMetricsFromMetricsAPI(ctx context.Context, ns string) (map[string]usageValues, bool) {
	if c.dyn == nil {
		return map[string]usageValues{}, false
	}
	podMetrics, err := c.dyn.Resource(schema.GroupVersionResource{
		Group:    "metrics.k8s.io",
		Version:  "v1beta1",
		Resource: "pods",
	}).Namespace(ns).List(ctx, metav1.ListOptions{})
	if err != nil || podMetrics == nil {
		return map[string]usageValues{}, false
	}
	usageByPod := make(map[string]usageValues, len(podMetrics.Items))
	for _, item := range podMetrics.Items {
		usageByPod[item.GetName()] = usageValuesFromMetricsAPI(item.Object)
	}
	return usageByPod, true
}

func usageValuesFromMetricsAPI(obj map[string]any) usageValues {
	containers, found, _ := unstructured.NestedSlice(obj, "containers")
	if !found {
		return usageValues{}
	}
	var cpuMilli int64
	var memoryBytes int64
	for _, rawContainer := range containers {
		container, ok := rawContainer.(map[string]any)
		if !ok {
			continue
		}
		usage, ok := container["usage"].(map[string]any)
		if !ok {
			continue
		}
		if cpuRaw, ok := usage["cpu"].(string); ok && cpuRaw != "" {
			if q, parseErr := resource.ParseQuantity(cpuRaw); parseErr == nil {
				cpuMilli += q.MilliValue()
			}
		}
		if memRaw, ok := usage["memory"].(string); ok && memRaw != "" {
			if q, parseErr := resource.ParseQuantity(memRaw); parseErr == nil {
				memoryBytes += q.Value()
			}
		}
	}
	return usageValues{cpuMilli: cpuMilli, memoryBytes: memoryBytes}
}

func (c *Client) collectAllPodUsageMetricsFromKubelet(ctx context.Context) (map[string]usageValues, bool) {
	nodes, err := c.core.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return map[string]usageValues{}, false
	}
	return c.collectAllPodUsageMetricsFromKubeletForNodes(ctx, nodes.Items)
}

func (c *Client) collectAllPodUsageMetricsFromKubeletForNodes(ctx context.Context, nodes []corev1.Node) (map[string]usageValues, bool) {
	if c.rest == nil {
		return map[string]usageValues{}, false
	}
	restClient := c.core.CoreV1().RESTClient()
	now := time.Now()
	usageByPod := make(map[string]usageValues)
	seenPods := make(map[string]struct{})
	directAvailable := false

	for _, node := range nodes {
		scrape, ok := c.scrapeNodeResourceMetrics(ctx, restClient, node.Name)
		if !ok {
			continue
		}
		directAvailable = true
		for key, memoryBytes := range scrape.podMemoryBytes {
			usage := usageByPod[key]
			usage.memoryBytes = memoryBytes
			usageByPod[key] = usage
			seenPods[key] = struct{}{}
		}
		for key, cpuSeconds := range scrape.podCPUSeconds {
			usage := usageByPod[key]
			if cpuMilli, ok := c.updatePodCPUCounter(key, cpuSeconds, now); ok {
				usage.cpuMilli = cpuMilli
			}
			usageByPod[key] = usage
			seenPods[key] = struct{}{}
		}
	}

	c.pruneResourceMetricsCache(now, seenPods, nil)
	return usageByPod, directAvailable
}

func (c *Client) collectNodeUsageMetrics(ctx context.Context) (map[string]usageValues, bool) {
	usageByNode, metricsAvailable := c.collectNodeUsageMetricsFromMetricsAPI(ctx)
	if metricsAvailable {
		return usageByNode, true
	}
	return c.collectNodeUsageMetricsFromKubelet(ctx)
}

func (c *Client) collectNodeUsageMetricsFromMetricsAPI(ctx context.Context) (map[string]usageValues, bool) {
	if c.dyn == nil {
		return map[string]usageValues{}, false
	}
	nodeMetrics, err := c.dyn.Resource(schema.GroupVersionResource{
		Group:    "metrics.k8s.io",
		Version:  "v1beta1",
		Resource: "nodes",
	}).List(ctx, metav1.ListOptions{})
	if err != nil || nodeMetrics == nil {
		return map[string]usageValues{}, false
	}
	usageByNode := make(map[string]usageValues, len(nodeMetrics.Items))
	for _, item := range nodeMetrics.Items {
		name := item.GetName()
		cpuRaw, _, _ := unstructured.NestedString(item.Object, "usage", "cpu")
		memRaw, _, _ := unstructured.NestedString(item.Object, "usage", "memory")
		usage := usageValues{}
		if cpuRaw != "" {
			if q, parseErr := resource.ParseQuantity(cpuRaw); parseErr == nil {
				usage.cpuMilli = q.MilliValue()
			}
		}
		if memRaw != "" {
			if q, parseErr := resource.ParseQuantity(memRaw); parseErr == nil {
				usage.memoryBytes = q.Value()
			}
		}
		usageByNode[name] = usage
	}
	return usageByNode, true
}

func (c *Client) collectNodeUsageMetricsFromKubelet(ctx context.Context) (map[string]usageValues, bool) {
	nodes, err := c.core.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return map[string]usageValues{}, false
	}
	return c.collectNodeUsageMetricsFromKubeletForNodes(ctx, nodes.Items)
}

func (c *Client) collectNodeUsageMetricsFromKubeletForNodes(ctx context.Context, nodes []corev1.Node) (map[string]usageValues, bool) {
	if c.rest == nil {
		return map[string]usageValues{}, false
	}
	restClient := c.core.CoreV1().RESTClient()
	now := time.Now()
	usageByNode := make(map[string]usageValues, len(nodes))
	seenNodes := make(map[string]struct{}, len(nodes))
	directAvailable := false

	for _, node := range nodes {
		scrape, ok := c.scrapeNodeResourceMetrics(ctx, restClient, node.Name)
		if !ok {
			continue
		}
		directAvailable = true
		usage := usageValues{}
		if scrape.hasNodeMemory {
			usage.memoryBytes = scrape.nodeMemoryBytes
		}
		if scrape.hasNodeCPU {
			if cpuMilli, ok := c.updateNodeCPUCounter(node.Name, scrape.nodeCPUSeconds, now); ok {
				usage.cpuMilli = cpuMilli
			}
		}
		usageByNode[node.Name] = usage
		seenNodes[node.Name] = struct{}{}
	}

	c.pruneResourceMetricsCache(now, nil, seenNodes)
	return usageByNode, directAvailable
}

func (c *Client) scrapeNodeResourceMetrics(ctx context.Context, restClient rest.Interface, nodeName string) (resourceMetricsScrape, bool) {
	path := fmt.Sprintf("/api/v1/nodes/%s/proxy/metrics/resource", nodeName)
	raw, err := restClient.Get().AbsPath(path).DoRaw(ctx)
	if err != nil {
		return resourceMetricsScrape{}, false
	}
	scrape, err := parseResourceMetrics(raw)
	if err != nil || scrape.resourceScrapeError {
		return resourceMetricsScrape{}, false
	}
	return scrape, true
}

func parseResourceMetrics(raw []byte) (resourceMetricsScrape, error) {
	result := resourceMetricsScrape{
		podCPUSeconds:  make(map[string]float64),
		podMemoryBytes: make(map[string]int64),
	}
	lines := strings.Split(string(raw), "\n")
	for _, line := range lines {
		name, labels, value, ok := parsePrometheusSample(line)
		if !ok {
			continue
		}
		switch name {
		case "resource_scrape_error":
			result.resourceScrapeError = value != 0
		case "pod_cpu_usage_seconds_total":
			ns := strings.TrimSpace(labels["namespace"])
			pod := strings.TrimSpace(labels["pod"])
			if ns != "" && pod != "" {
				result.podCPUSeconds[ns+"/"+pod] = value
			}
		case "pod_memory_working_set_bytes":
			ns := strings.TrimSpace(labels["namespace"])
			pod := strings.TrimSpace(labels["pod"])
			if ns != "" && pod != "" {
				result.podMemoryBytes[ns+"/"+pod] = int64(value)
			}
		case "node_cpu_usage_seconds_total":
			result.nodeCPUSeconds = value
			result.hasNodeCPU = true
		case "node_memory_working_set_bytes":
			result.nodeMemoryBytes = int64(value)
			result.hasNodeMemory = true
		}
	}
	return result, nil
}

func parsePrometheusSample(line string) (string, map[string]string, float64, bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", nil, 0, false
	}
	valueSep := -1
	inQuotes := false
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '"':
			inQuotes = !inQuotes
		case ' ', '\t':
			if !inQuotes {
				valueSep = i
				i = len(line)
			}
		}
	}
	if valueSep < 0 {
		return "", nil, 0, false
	}
	metricPart := strings.TrimSpace(line[:valueSep])
	valuePart := strings.TrimSpace(line[valueSep+1:])
	if nextSep := strings.IndexByte(valuePart, ' '); nextSep >= 0 {
		valuePart = valuePart[:nextSep]
	}
	value, err := strconv.ParseFloat(valuePart, 64)
	if err != nil {
		return "", nil, 0, false
	}
	if braceIdx := strings.IndexByte(metricPart, '{'); braceIdx >= 0 {
		name := metricPart[:braceIdx]
		if !strings.HasSuffix(metricPart, "}") {
			return "", nil, 0, false
		}
		labels, err := parsePrometheusLabels(metricPart[braceIdx+1 : len(metricPart)-1])
		if err != nil {
			return "", nil, 0, false
		}
		return name, labels, value, true
	}
	return metricPart, map[string]string{}, value, true
}

func parsePrometheusLabels(input string) (map[string]string, error) {
	labels := make(map[string]string)
	for len(input) > 0 {
		eqIdx := strings.IndexByte(input, '=')
		if eqIdx <= 0 || eqIdx+1 >= len(input) || input[eqIdx+1] != '"' {
			return nil, fmt.Errorf("invalid labels")
		}
		key := strings.TrimSpace(input[:eqIdx])
		input = input[eqIdx+2:]
		var value strings.Builder
		escaped := false
		endIdx := -1
		for i := 0; i < len(input); i++ {
			ch := input[i]
			if escaped {
				value.WriteByte(ch)
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				endIdx = i
				break
			}
			value.WriteByte(ch)
		}
		if endIdx < 0 {
			return nil, fmt.Errorf("unterminated label value")
		}
		labels[key] = value.String()
		input = input[endIdx+1:]
		if len(input) == 0 {
			break
		}
		if input[0] != ',' {
			return nil, fmt.Errorf("invalid label separator")
		}
		input = strings.TrimLeft(input[1:], " ")
	}
	return labels, nil
}

func (c *Client) updatePodCPUCounter(key string, current float64, now time.Time) (int64, bool) {
	return c.updateCPUCounter(c.metrics.podCPU, key, current, now)
}

func (c *Client) updateNodeCPUCounter(key string, current float64, now time.Time) (int64, bool) {
	return c.updateCPUCounter(c.metrics.nodeCPU, key, current, now)
}

func (c *Client) updateCPUCounter(cache map[string]resourceCounterSample, key string, current float64, now time.Time) (int64, bool) {
	c.metrics.mu.Lock()
	defer c.metrics.mu.Unlock()

	prev, ok := cache[key]
	cache[key] = resourceCounterSample{value: current, sampledAt: now}
	if !ok {
		return 0, false
	}
	delta := current - prev.value
	elapsed := now.Sub(prev.sampledAt).Seconds()
	if delta < 0 || elapsed <= 0 {
		return 0, false
	}
	return int64(math.Round((delta / elapsed) * 1000)), true
}

func (c *Client) pruneResourceMetricsCache(now time.Time, seenPods, seenNodes map[string]struct{}) {
	const ttl = 30 * time.Minute

	c.metrics.mu.Lock()
	defer c.metrics.mu.Unlock()

	if seenPods != nil {
		for key, sample := range c.metrics.podCPU {
			if _, ok := seenPods[key]; !ok && now.Sub(sample.sampledAt) > ttl {
				delete(c.metrics.podCPU, key)
			}
		}
	}
	if seenNodes != nil {
		for key, sample := range c.metrics.nodeCPU {
			if _, ok := seenNodes[key]; !ok && now.Sub(sample.sampledAt) > ttl {
				delete(c.metrics.nodeCPU, key)
			}
		}
	}
}

func (c *Client) resourceMetricsStatusForNodes(ctx context.Context, nodes []corev1.Node) resourceMetricsStatus {
	if _, ok := c.collectNodeUsageMetricsFromMetricsAPI(ctx); ok {
		return resourceMetricsStatus{metricsServerAvailable: true, directAvailable: true}
	}
	_, directAvailable := c.collectNodeUsageMetricsFromKubeletForNodes(ctx, nodes)
	return resourceMetricsStatus{metricsServerAvailable: false, directAvailable: directAvailable}
}

func (c *Client) collectDCGMMetrics(ctx context.Context) dcgmMetricsSnapshot {
	now := time.Now()

	c.dcgm.mu.Lock()
	if now.Before(c.dcgm.expiresAt) {
		snapshot := c.dcgm.snapshot
		c.dcgm.mu.Unlock()
		return snapshot
	}
	c.dcgm.mu.Unlock()

	snapshot := c.scrapeDCGMMetrics(ctx)

	c.dcgm.mu.Lock()
	c.dcgm.snapshot = snapshot
	c.dcgm.expiresAt = time.Now().Add(15 * time.Second)
	c.dcgm.mu.Unlock()
	return snapshot
}

func (c *Client) scrapeDCGMMetrics(ctx context.Context) dcgmMetricsSnapshot {
	pods, err := c.listDCGMExporterPods(ctx)
	if err != nil || len(pods) == 0 {
		return dcgmMetricsSnapshot{
			nodeUsage: map[string]gpuUsageValues{},
			podUsage:  map[string]gpuUsageValues{},
		}
	}

	nodeAggregates := make(map[string]*dcgmUsageAggregate)
	podAggregates := make(map[string]*dcgmUsageAggregate)
	var (
		mu        sync.Mutex
		wg        sync.WaitGroup
		available bool
	)

	for _, pod := range pods {
		if strings.TrimSpace(pod.Status.PodIP) == "" || strings.TrimSpace(pod.Spec.NodeName) == "" {
			continue
		}
		wg.Add(1)
		go func(exporterPod corev1.Pod) {
			defer wg.Done()
			raw, err := c.fetchDCGMMetricsFromPod(ctx, exporterPod)
			if err != nil || len(raw) == 0 {
				return
			}
			nodeData, podData, ok := parseDCGMMetrics(raw, exporterPod.Spec.NodeName)
			if !ok {
				return
			}
			mu.Lock()
			defer mu.Unlock()
			available = true
			for nodeName, aggregate := range nodeData {
				target := nodeAggregates[nodeName]
				if target == nil {
					target = &dcgmUsageAggregate{entities: make(map[string]dcgmEntityUsage)}
					nodeAggregates[nodeName] = target
				}
				target.merge(aggregate)
			}
			for podKey, aggregate := range podData {
				target := podAggregates[podKey]
				if target == nil {
					target = &dcgmUsageAggregate{entities: make(map[string]dcgmEntityUsage)}
					podAggregates[podKey] = target
				}
				target.merge(aggregate)
			}
		}(pod)
	}
	wg.Wait()

	snapshot := dcgmMetricsSnapshot{
		available: available,
		nodeUsage: make(map[string]gpuUsageValues, len(nodeAggregates)),
		podUsage:  make(map[string]gpuUsageValues, len(podAggregates)),
	}
	for key, aggregate := range nodeAggregates {
		snapshot.nodeUsage[key] = aggregate.finalize()
	}
	for key, aggregate := range podAggregates {
		snapshot.podUsage[key] = aggregate.finalize()
	}
	return snapshot
}

func (c *Client) listDCGMExporterPods(ctx context.Context) ([]corev1.Pod, error) {
	selectors := []string{
		"app.kubernetes.io/name=dcgm-exporter",
		"app=dcgm-exporter",
	}
	for _, selector := range selectors {
		items, err := c.core.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{LabelSelector: selector})
		if err != nil {
			continue
		}
		pods := filterRunningDCGMExporterPods(items.Items)
		if len(pods) > 0 {
			return pods, nil
		}
	}
	items, err := c.core.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return filterRunningDCGMExporterPods(items.Items), nil
}

func filterRunningDCGMExporterPods(items []corev1.Pod) []corev1.Pod {
	out := make([]corev1.Pod, 0)
	for _, pod := range items {
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}
		if strings.TrimSpace(pod.Status.PodIP) == "" {
			continue
		}
		if isDCGMExporterPod(pod) {
			out = append(out, pod)
		}
	}
	return out
}

func isDCGMExporterPod(pod corev1.Pod) bool {
	name := strings.ToLower(strings.TrimSpace(pod.Name))
	if strings.Contains(name, "dcgm-exporter") {
		return true
	}
	for key, value := range pod.Labels {
		k := strings.ToLower(strings.TrimSpace(key))
		v := strings.ToLower(strings.TrimSpace(value))
		if (k == "app" || k == "app.kubernetes.io/name" || k == "app.kubernetes.io/component") && strings.Contains(v, "dcgm-exporter") {
			return true
		}
	}
	return false
}

func (c *Client) fetchDCGMMetricsFromPod(ctx context.Context, pod corev1.Pod) ([]byte, error) {
	port := dcgmMetricsPort(pod)
	client := &http.Client{Timeout: 3 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://%s:%d/metrics", pod.Status.PodIP, port), nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("dcgm-exporter returned %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func dcgmMetricsPort(pod corev1.Pod) int32 {
	for _, container := range pod.Spec.Containers {
		for _, port := range container.Ports {
			if port.ContainerPort == 9400 || strings.EqualFold(port.Name, "metrics") {
				return port.ContainerPort
			}
		}
	}
	return 9400
}

func parseDCGMMetrics(raw []byte, nodeName string) (map[string]*dcgmUsageAggregate, map[string]*dcgmUsageAggregate, bool) {
	nodeAggregates := make(map[string]*dcgmUsageAggregate)
	podAggregates := make(map[string]*dcgmUsageAggregate)
	found := false

	for _, line := range strings.Split(string(raw), "\n") {
		name, labels, value, ok := parsePrometheusSample(line)
		if !ok {
			continue
		}
		if name != "DCGM_FI_DEV_GPU_UTIL" && name != "DCGM_FI_DEV_FB_USED" {
			continue
		}
		entityKey, isMIG := dcgmEntityKey(labels)
		if entityKey == "" {
			continue
		}
		found = true

		nodeAggregate := nodeAggregates[nodeName]
		if nodeAggregate == nil {
			nodeAggregate = &dcgmUsageAggregate{entities: make(map[string]dcgmEntityUsage)}
			nodeAggregates[nodeName] = nodeAggregate
		}
		nodeAggregate.record(entityKey, isMIG, name, value)

		ns := strings.TrimSpace(labels["namespace"])
		pod := strings.TrimSpace(labels["pod"])
		if ns != "" && pod != "" {
			podKey := ns + "/" + pod
			podAggregate := podAggregates[podKey]
			if podAggregate == nil {
				podAggregate = &dcgmUsageAggregate{entities: make(map[string]dcgmEntityUsage)}
				podAggregates[podKey] = podAggregate
			}
			podAggregate.record(entityKey, isMIG, name, value)
		}
	}

	return nodeAggregates, podAggregates, found
}

func dcgmEntityKey(labels map[string]string) (string, bool) {
	base := strings.TrimSpace(labels["UUID"])
	if base == "" {
		base = strings.TrimSpace(labels["gpu"])
	}
	if base == "" {
		return "", false
	}
	if migID := strings.TrimSpace(labels["GPU_I_ID"]); migID != "" {
		return base + "/mig/" + migID, true
	}
	return base, false
}

func (a *dcgmUsageAggregate) record(entityKey string, isMIG bool, metric string, value float64) {
	if a.entities == nil {
		a.entities = make(map[string]dcgmEntityUsage)
	}
	entity := a.entities[entityKey]
	entity.mig = isMIG
	switch metric {
	case "DCGM_FI_DEV_GPU_UTIL":
		entity.hasUtil = true
		entity.utilPercent = value
	case "DCGM_FI_DEV_FB_USED":
		entity.hasFBUsed = true
		entity.fbUsedMiB = value
	}
	a.entities[entityKey] = entity
}

func (a *dcgmUsageAggregate) merge(other *dcgmUsageAggregate) {
	if other == nil {
		return
	}
	if a.entities == nil {
		a.entities = make(map[string]dcgmEntityUsage, len(other.entities))
	}
	for key, entity := range other.entities {
		current := a.entities[key]
		if entity.hasUtil {
			current.hasUtil = true
			current.utilPercent = entity.utilPercent
		}
		if entity.hasFBUsed {
			current.hasFBUsed = true
			current.fbUsedMiB = entity.fbUsedMiB
		}
		current.mig = current.mig || entity.mig
		a.entities[key] = current
	}
}

func (a *dcgmUsageAggregate) finalize() gpuUsageValues {
	if a == nil || len(a.entities) == 0 {
		return gpuUsageValues{}
	}
	hasNonMIG := false
	for _, entity := range a.entities {
		if !entity.mig {
			hasNonMIG = true
			break
		}
	}

	var (
		utilSum        float64
		utilCount      int64
		memoryUsedMiB  float64
		deviceCount    int64
		hasMemoryUsage bool
	)
	for _, entity := range a.entities {
		if hasNonMIG && entity.mig {
			continue
		}
		deviceCount++
		if entity.hasUtil {
			utilSum += entity.utilPercent
			utilCount++
		}
		if entity.hasFBUsed {
			memoryUsedMiB += entity.fbUsedMiB
			hasMemoryUsage = true
		}
	}

	usage := gpuUsageValues{
		deviceCount: deviceCount,
		hasUtil:     utilCount > 0,
		hasMemory:   hasMemoryUsage,
	}
	if utilCount > 0 {
		usage.utilPercent = int64(math.Round(utilSum / float64(utilCount)))
	}
	if hasMemoryUsage {
		usage.memoryUsedBytes = int64(math.Round(memoryUsedMiB * 1024 * 1024))
	}
	return usage
}
