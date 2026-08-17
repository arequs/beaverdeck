package kube

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

const (
	restartDiagnosticVersion       = 1
	restartDiagnosticSecretKey     = "incident.json"
	restartDiagnosticSecretPurpose = "restart-diagnostic"
	restartDiagnosticQueueSize     = 128
	restartDiagnosticWorkers       = 2
	restartDiagnosticNodePodLimit  = 100
	restartDiagnosticDedupeSize    = 4096
)

var restartDiagnosticOffsets = []time.Duration{-3 * time.Minute, -time.Minute, -30 * time.Second, -10 * time.Second}

type RestartDiagnosticsOptions struct {
	Enabled                 bool
	Namespace               string
	Interval                time.Duration
	History                 time.Duration
	MaxLogLines             int64
	MaxLogBytesPerContainer int64
	MaxTotalLogBytes        int64
	MaxEvents               int
}

func (o RestartDiagnosticsOptions) withDefaults() RestartDiagnosticsOptions {
	if o.Interval <= 0 {
		o.Interval = 10 * time.Second
	}
	if o.History < 5*time.Minute {
		o.History = 5 * time.Minute
	}
	if o.MaxLogLines <= 0 {
		o.MaxLogLines = 100
	}
	if o.MaxLogBytesPerContainer <= 0 {
		o.MaxLogBytesPerContainer = 32768
	}
	if o.MaxTotalLogBytes <= 0 {
		o.MaxTotalLogBytes = 131072
	}
	if o.MaxEvents <= 0 {
		o.MaxEvents = 20
	}
	return o
}

type RestartDiagnosticWorkload struct {
	APIVersion string    `json:"api_version"`
	Kind       string    `json:"kind"`
	Namespace  string    `json:"namespace"`
	Name       string    `json:"name"`
	UID        types.UID `json:"uid,omitempty"`
}

type RestartDiagnosticPod struct {
	Namespace    string    `json:"namespace"`
	Name         string    `json:"name"`
	UID          types.UID `json:"uid"`
	Node         string    `json:"node,omitempty"`
	RestartCount int32     `json:"restart_count"`
}

type RestartDiagnosticResources struct {
	CPURequestMilli    int64 `json:"cpu_request_milli"`
	CPULimitMilli      int64 `json:"cpu_limit_milli"`
	MemoryRequestBytes int64 `json:"memory_request_bytes"`
	MemoryLimitBytes   int64 `json:"memory_limit_bytes"`
}

type RestartDiagnosticContainer struct {
	Name         string                     `json:"name"`
	Init         bool                       `json:"init"`
	RestartCount int32                      `json:"restart_count"`
	Resources    RestartDiagnosticResources `json:"resources"`
}

type RestartDiagnosticMetricPoint struct {
	OffsetSeconds int64     `json:"offset_seconds"`
	TargetTime    time.Time `json:"target_time"`
	SampledAt     time.Time `json:"sampled_at,omitempty"`
	Available     bool      `json:"available"`
	CPUUsedMilli  int64     `json:"cpu_used_milli,omitempty"`
	MemoryBytes   int64     `json:"memory_bytes,omitempty"`
	Source        string    `json:"source,omitempty"`
}

type RestartDiagnosticNodePod struct {
	Namespace       string `json:"namespace"`
	Pod             string `json:"pod"`
	Container       string `json:"container,omitempty"`
	CPUUsedMilli    int64  `json:"cpu_used_milli"`
	MemoryUsedBytes int64  `json:"memory_used_bytes"`
	Affected        bool   `json:"affected"`
}

type RestartDiagnosticNodeResources struct {
	MetricsAvailable       bool      `json:"metrics_available"`
	SampledAt              time.Time `json:"sampled_at,omitempty"`
	Source                 string    `json:"source,omitempty"`
	CPUUsedMilli           int64     `json:"cpu_used_milli,omitempty"`
	CPUAllocatableMilli    int64     `json:"cpu_allocatable_milli"`
	CPUCapacityMilli       int64     `json:"cpu_capacity_milli"`
	MemoryUsedBytes        int64     `json:"memory_used_bytes,omitempty"`
	MemoryAllocatableBytes int64     `json:"memory_allocatable_bytes"`
	MemoryCapacityBytes    int64     `json:"memory_capacity_bytes"`
}

type RestartDiagnosticPersistentStorage struct {
	Volume         string   `json:"volume"`
	ClaimNamespace string   `json:"claim_namespace"`
	ClaimName      string   `json:"claim_name"`
	ClaimStatus    string   `json:"claim_status,omitempty"`
	RequestedBytes int64    `json:"requested_bytes,omitempty"`
	CapacityBytes  int64    `json:"capacity_bytes,omitempty"`
	StorageClass   string   `json:"storage_class,omitempty"`
	VolumeName     string   `json:"volume_name,omitempty"`
	VolumeStatus   string   `json:"volume_status,omitempty"`
	AccessModes    []string `json:"access_modes,omitempty"`
	VolumeMode     string   `json:"volume_mode,omitempty"`
	Error          string   `json:"error,omitempty"`
}

type RestartDiagnosticLog struct {
	Container string `json:"container"`
	Init      bool   `json:"init"`
	Available bool   `json:"available"`
	Truncated bool   `json:"truncated,omitempty"`
	Text      string `json:"text,omitempty"`
	Error     string `json:"error,omitempty"`
}

type RestartDiagnosticEvent struct {
	Time    time.Time `json:"time"`
	Type    string    `json:"type"`
	Reason  string    `json:"reason"`
	Object  string    `json:"object"`
	Message string    `json:"message"`
	Count   int32     `json:"count"`
}

type RestartDiagnosticSnapshot struct {
	Version           int                                  `json:"version"`
	IncidentTime      time.Time                            `json:"incident_time"`
	Classification    string                               `json:"classification"`
	Reason            string                               `json:"reason,omitempty"`
	Message           string                               `json:"message,omitempty"`
	ExitCode          *int32                               `json:"exit_code,omitempty"`
	Workload          RestartDiagnosticWorkload            `json:"workload"`
	Pod               RestartDiagnosticPod                 `json:"pod"`
	Container         RestartDiagnosticContainer           `json:"container"`
	ContainerMetrics  []RestartDiagnosticMetricPoint       `json:"container_metrics"`
	NodeMetrics       []RestartDiagnosticMetricPoint       `json:"node_metrics"`
	NodeResources     RestartDiagnosticNodeResources       `json:"node_resources"`
	NodePods          []RestartDiagnosticNodePod           `json:"node_pods"`
	PersistentStorage []RestartDiagnosticPersistentStorage `json:"persistent_storage"`
	PreviousLogs      []RestartDiagnosticLog               `json:"previous_logs"`
	Events            []RestartDiagnosticEvent             `json:"events"`
	StorageEvents     []RestartDiagnosticEvent             `json:"storage_events"`
	Warnings          []string                             `json:"warnings,omitempty"`
}

type RestartDiagnosticSummary struct {
	SecretName     string                    `json:"secret_name"`
	IncidentTime   time.Time                 `json:"incident_time"`
	Classification string                    `json:"classification"`
	Reason         string                    `json:"reason,omitempty"`
	Workload       RestartDiagnosticWorkload `json:"workload"`
	Pod            RestartDiagnosticPod      `json:"pod"`
	Container      string                    `json:"container"`
	RestartCount   int32                     `json:"restart_count"`
}

type restartIncident struct {
	pod          *corev1.Pod
	container    string
	init         bool
	restartCount int32
	terminated   *corev1.ContainerStateTerminated
	evicted      bool
	at           time.Time
}

type diagnosticPodMetric struct {
	Namespace string
	Pod       string
	Node      string
	Usage     usageValues
}

type restartMetricSample struct {
	At         time.Time
	Source     string
	Containers map[string]usageValues
	Pods       map[string]diagnosticPodMetric
	Nodes      map[string]usageValues
}

type restartMetricRing struct {
	mu       sync.RWMutex
	items    []restartMetricSample
	capacity int
}

func newRestartMetricRing(history, interval time.Duration) *restartMetricRing {
	capacity := int(history/interval) + 3
	if capacity < 4 {
		capacity = 4
	}
	return &restartMetricRing{capacity: capacity}
}

func (r *restartMetricRing) add(sample restartMetricSample) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.items) == r.capacity {
		copy(r.items, r.items[1:])
		r.items[len(r.items)-1] = sample
		return
	}
	r.items = append(r.items, sample)
}

func (r *restartMetricRing) closestAtOrBefore(target time.Time) (restartMetricSample, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for i := len(r.items) - 1; i >= 0; i-- {
		if !r.items[i].At.After(target) {
			return r.items[i], true
		}
	}
	return restartMetricSample{}, false
}

func (r *restartMetricRing) closest(target, notAfter time.Time, tolerance time.Duration) (restartMetricSample, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	best := restartMetricSample{}
	bestDistance := time.Duration(1<<63 - 1)
	found := false
	for i := range r.items {
		if r.items[i].At.After(notAfter) {
			continue
		}
		distance := r.items[i].At.Sub(target)
		if distance < 0 {
			distance = -distance
		}
		if distance < bestDistance {
			best = r.items[i]
			bestDistance = distance
			found = true
		}
	}
	return best, found && bestDistance <= tolerance
}

type RestartDiagnostics struct {
	client   *Client
	opts     RestartDiagnosticsOptions
	ring     *restartMetricRing
	queue    chan restartIncident
	podStore cache.Store
	now      func() time.Time
	ready    atomic.Bool

	dedupeMu    sync.Mutex
	dedupe      map[string]struct{}
	dedupeOrder []string
}

func (c *Client) StartRestartDiagnostics(ctx context.Context, opts RestartDiagnosticsOptions) {
	opts = opts.withDefaults()
	if !opts.Enabled {
		return
	}
	d := &RestartDiagnostics{
		client: c,
		opts:   opts,
		ring:   newRestartMetricRing(opts.History, opts.Interval),
		queue:  make(chan restartIncident, restartDiagnosticQueueSize),
		now:    time.Now,
		dedupe: make(map[string]struct{}),
	}
	d.start(ctx)
}

func (d *RestartDiagnostics) start(ctx context.Context) {
	factoryOptions := []informers.SharedInformerOption{}
	if strings.TrimSpace(d.opts.Namespace) != "" {
		factoryOptions = append(factoryOptions, informers.WithNamespace(d.opts.Namespace))
	}
	factory := informers.NewSharedInformerFactoryWithOptions(d.client.core, 0, factoryOptions...)
	podInformer := factory.Core().V1().Pods().Informer()
	d.podStore = podInformer.GetStore()
	_, _ = podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			if !d.ready.Load() {
				return
			}
			pod, ok := obj.(*corev1.Pod)
			if !ok {
				return
			}
			for _, incident := range detectRestartIncidents(nil, pod, false, d.now()) {
				d.enqueue(incident)
			}
		},
		UpdateFunc: func(oldObj, newObj any) {
			if !d.ready.Load() {
				return
			}
			oldPod, oldOK := oldObj.(*corev1.Pod)
			newPod, newOK := newObj.(*corev1.Pod)
			if !oldOK || !newOK {
				return
			}
			for _, incident := range detectRestartIncidents(oldPod, newPod, false, d.now()) {
				d.enqueue(incident)
			}
		},
		DeleteFunc: func(obj any) {
			if !d.ready.Load() {
				return
			}
			pod, ok := obj.(*corev1.Pod)
			if !ok {
				if tombstone, tombstoneOK := obj.(cache.DeletedFinalStateUnknown); tombstoneOK {
					pod, ok = tombstone.Obj.(*corev1.Pod)
				}
			}
			if !ok || pod == nil {
				return
			}
			for _, incident := range detectRestartIncidents(nil, pod, true, d.now()) {
				d.enqueue(incident)
			}
		},
	})

	for i := 0; i < restartDiagnosticWorkers; i++ {
		go d.worker(ctx)
	}
	factory.Start(ctx.Done())
	go func() {
		if !cache.WaitForCacheSync(ctx.Done(), podInformer.HasSynced) {
			log.Printf("restart diagnostics: pod informer cache did not sync")
			return
		}
		d.sample(ctx)
		d.ready.Store(true)
		if err := d.client.migrateRestartDiagnosticSecrets(ctx, d.opts.Namespace); err != nil && ctx.Err() == nil {
			log.Printf("restart diagnostics: existing Secret migration failed: %v", err)
		}
		ticker := time.NewTicker(d.opts.Interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				d.sample(ctx)
			}
		}
	}()
	log.Printf("restart diagnostics enabled: interval=%s history=%s namespace=%q", d.opts.Interval, d.opts.History, d.opts.Namespace)
}

func (d *RestartDiagnostics) enqueue(incident restartIncident) {
	key := restartIncidentKey(incident)
	d.dedupeMu.Lock()
	if _, exists := d.dedupe[key]; exists {
		d.dedupeMu.Unlock()
		return
	}
	d.dedupe[key] = struct{}{}
	d.dedupeMu.Unlock()

	select {
	case d.queue <- incident:
		d.dedupeMu.Lock()
		d.dedupeOrder = append(d.dedupeOrder, key)
		if len(d.dedupeOrder) > restartDiagnosticDedupeSize {
			oldest := d.dedupeOrder[0]
			d.dedupeOrder = d.dedupeOrder[1:]
			delete(d.dedupe, oldest)
		}
		d.dedupeMu.Unlock()
	default:
		d.dedupeMu.Lock()
		delete(d.dedupe, key)
		d.dedupeMu.Unlock()
		log.Printf("restart diagnostics: incident queue full, dropped %s/%s container=%s", incident.pod.Namespace, incident.pod.Name, incident.container)
	}
}

func (d *RestartDiagnostics) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case incident := <-d.queue:
			var err error
			for attempt := 0; attempt < 3; attempt++ {
				err = d.capture(ctx, incident)
				if err == nil || ctx.Err() != nil {
					break
				}
			}
			if err != nil && ctx.Err() == nil {
				log.Printf("restart diagnostics: capture failed for %s/%s container=%s: %v", incident.pod.Namespace, incident.pod.Name, incident.container, err)
			}
		}
	}
}

func restartIncidentKey(incident restartIncident) string {
	finished := int64(0)
	if incident.terminated != nil {
		finished = incident.terminated.FinishedAt.UnixNano()
	}
	return fmt.Sprintf("%s/%d/%d/%t", incident.pod.UID, podRestartCount(incident.pod), finished, incident.evicted)
}

func detectRestartIncidents(oldPod, pod *corev1.Pod, deleted bool, now time.Time) []restartIncident {
	if pod == nil {
		return nil
	}
	if deleted {
		if !strings.EqualFold(pod.Status.Reason, "Evicted") {
			return nil
		}
		return evictionIncidents(pod, now)
	}
	if strings.EqualFold(pod.Status.Reason, "Evicted") && (oldPod == nil || !strings.EqualFold(oldPod.Status.Reason, "Evicted")) {
		return evictionIncidents(pod, now)
	}

	oldRegular := statusRestartCounts(nil)
	oldInit := statusRestartCounts(nil)
	if oldPod != nil {
		oldRegular = statusRestartCounts(oldPod.Status.ContainerStatuses)
		oldInit = statusRestartCounts(oldPod.Status.InitContainerStatuses)
	}
	incidents := make([]restartIncident, 0)
	appendChanged := func(statuses []corev1.ContainerStatus, previous map[string]int32, init bool) {
		for _, status := range statuses {
			if status.RestartCount <= previous[status.Name] {
				continue
			}
			at := now
			var termination *corev1.ContainerStateTerminated
			if status.LastTerminationState.Terminated != nil {
				termination = status.LastTerminationState.Terminated.DeepCopy()
				if !termination.FinishedAt.IsZero() {
					at = termination.FinishedAt.Time
				}
			}
			incidents = append(incidents, restartIncident{
				pod: pod.DeepCopy(), container: status.Name, init: init,
				restartCount: status.RestartCount, terminated: termination, at: at,
			})
		}
	}
	appendChanged(pod.Status.ContainerStatuses, oldRegular, false)
	appendChanged(pod.Status.InitContainerStatuses, oldInit, true)
	if len(incidents) <= 1 {
		return incidents
	}
	// A restart transition belongs to the pod. Keep one incident focus while
	// capture still includes previous logs from every container in that pod.
	sort.SliceStable(incidents, func(i, j int) bool {
		if !incidents[i].at.Equal(incidents[j].at) {
			return incidents[i].at.After(incidents[j].at)
		}
		iOOM := incidents[i].terminated != nil && strings.EqualFold(incidents[i].terminated.Reason, "OOMKilled")
		jOOM := incidents[j].terminated != nil && strings.EqualFold(incidents[j].terminated.Reason, "OOMKilled")
		if iOOM != jOOM {
			return iOOM
		}
		return !incidents[i].init && incidents[j].init
	})
	return incidents[:1]
}

func statusRestartCounts(statuses []corev1.ContainerStatus) map[string]int32 {
	out := make(map[string]int32, len(statuses))
	for _, status := range statuses {
		out[status.Name] = status.RestartCount
	}
	return out
}

func evictionIncidents(pod *corev1.Pod, now time.Time) []restartIncident {
	incident := restartIncident{pod: pod.DeepCopy(), container: "pod", evicted: true, at: now}
	if len(pod.Spec.Containers) > 0 {
		incident.container = pod.Spec.Containers[0].Name
	}
	if status := findContainerStatus(pod.Status.ContainerStatuses, incident.container); status != nil {
		incident.restartCount = status.RestartCount
		if status.LastTerminationState.Terminated != nil {
			incident.terminated = status.LastTerminationState.Terminated.DeepCopy()
		}
	}
	return []restartIncident{incident}
}

func findContainerStatus(statuses []corev1.ContainerStatus, name string) *corev1.ContainerStatus {
	for i := range statuses {
		if statuses[i].Name == name {
			return &statuses[i]
		}
	}
	return nil
}

func (d *RestartDiagnostics) sample(ctx context.Context) {
	sample := d.client.collectRestartDiagnosticMetrics(ctx, d.opts.Namespace, d.podStore, d.now())
	d.ring.add(sample)
}

func (c *Client) collectRestartDiagnosticMetrics(ctx context.Context, namespace string, podStore cache.Store, now time.Time) restartMetricSample {
	sample := restartMetricSample{
		At: now, Containers: make(map[string]usageValues), Pods: make(map[string]diagnosticPodMetric), Nodes: make(map[string]usageValues),
	}
	podMetricsAvailable := false
	nodeMetricsAvailable := false
	if c.dyn != nil {
		resource := c.dyn.Resource(schema.GroupVersionResource{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "pods"})
		var list *unstructured.UnstructuredList
		var err error
		if namespace == "" {
			list, err = resource.List(ctx, metav1.ListOptions{})
		} else {
			list, err = resource.Namespace(namespace).List(ctx, metav1.ListOptions{})
		}
		if err == nil && list != nil {
			podMetricsAvailable = true
			for _, item := range list.Items {
				ns := item.GetNamespace()
				podName := item.GetName()
				containers := containerUsageValuesFromMetricsAPI(item.Object)
				aggregate := usageValues{}
				for name, usage := range containers {
					sample.Containers[containerMetricKey(ns, podName, name)] = usage
					aggregate.cpuMilli += usage.cpuMilli
					aggregate.memoryBytes += usage.memoryBytes
				}
				sample.Pods[podMetricKey(ns, podName)] = diagnosticPodMetric{Namespace: ns, Pod: podName, Node: podNodeFromStore(podStore, ns, podName), Usage: aggregate}
			}
		}
		nodeList, err := c.dyn.Resource(schema.GroupVersionResource{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "nodes"}).List(ctx, metav1.ListOptions{})
		if err == nil && nodeList != nil {
			nodeMetricsAvailable = true
			for _, item := range nodeList.Items {
				sample.Nodes[item.GetName()] = nodeUsageValuesFromMetricsAPI(item.Object)
			}
		}
	}
	if podMetricsAvailable && nodeMetricsAvailable {
		sample.Source = "metrics-server"
		return sample
	}
	if c.rest == nil || c.core == nil {
		if podMetricsAvailable || nodeMetricsAvailable {
			sample.Source = "metrics-server-partial"
		}
		return sample
	}
	nodes, err := c.core.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		if podMetricsAvailable || nodeMetricsAvailable {
			sample.Source = "metrics-server-partial"
		}
		return sample
	}
	restClient := c.core.CoreV1().RESTClient()
	directAvailable := false
	seenContainers := make(map[string]struct{})
	for _, node := range nodes.Items {
		scrape, ok := c.scrapeNodeResourceMetrics(ctx, restClient, node.Name)
		if !ok {
			continue
		}
		directAvailable = true
		if !podMetricsAvailable {
			for key, memory := range scrape.containerMemoryBytes {
				usage := sample.Containers[key]
				usage.memoryBytes = memory
				sample.Containers[key] = usage
				seenContainers[key] = struct{}{}
			}
			for key, seconds := range scrape.containerCPUSeconds {
				usage := sample.Containers[key]
				if milli, available := c.updateContainerCPUCounter(key, seconds, now); available {
					usage.cpuMilli = milli
				}
				sample.Containers[key] = usage
				seenContainers[key] = struct{}{}
			}
			for key, memory := range scrape.podMemoryBytes {
				podMetric := sample.Pods[key]
				podMetric.Namespace, podMetric.Pod = splitPodMetricKey(key)
				podMetric.Node = node.Name
				podMetric.Usage.memoryBytes = memory
				sample.Pods[key] = podMetric
			}
			for key, seconds := range scrape.podCPUSeconds {
				podMetric := sample.Pods[key]
				podMetric.Namespace, podMetric.Pod = splitPodMetricKey(key)
				podMetric.Node = node.Name
				if milli, available := c.updatePodCPUCounter(key, seconds, now); available {
					podMetric.Usage.cpuMilli = milli
				}
				sample.Pods[key] = podMetric
			}
		}
		if !nodeMetricsAvailable {
			usage := sample.Nodes[node.Name]
			if scrape.hasNodeMemory {
				usage.memoryBytes = scrape.nodeMemoryBytes
			}
			if scrape.hasNodeCPU {
				if milli, available := c.updateNodeCPUCounter(node.Name, scrape.nodeCPUSeconds, now); available {
					usage.cpuMilli = milli
				}
			}
			sample.Nodes[node.Name] = usage
		}
	}
	if !podMetricsAvailable {
		c.pruneContainerResourceMetricsCache(now, seenContainers)
	}
	switch {
	case directAvailable && (podMetricsAvailable || nodeMetricsAvailable):
		sample.Source = "metrics-server+kubelet"
	case directAvailable:
		sample.Source = "kubelet"
	case podMetricsAvailable || nodeMetricsAvailable:
		sample.Source = "metrics-server-partial"
	}
	return sample
}

func (c *Client) pruneContainerResourceMetricsCache(now time.Time, seen map[string]struct{}) {
	const ttl = 30 * time.Minute
	c.metrics.mu.Lock()
	defer c.metrics.mu.Unlock()
	for key, item := range c.metrics.containerCPU {
		if _, ok := seen[key]; !ok && now.Sub(item.sampledAt) > ttl {
			delete(c.metrics.containerCPU, key)
		}
	}
}

func containerUsageValuesFromMetricsAPI(obj map[string]any) map[string]usageValues {
	out := make(map[string]usageValues)
	containers, found, _ := unstructured.NestedSlice(obj, "containers")
	if !found {
		return out
	}
	for _, raw := range containers {
		container, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		name, _ := container["name"].(string)
		usage, _ := container["usage"].(map[string]any)
		out[name] = usageValuesFromAnyQuantityMap(usage)
	}
	return out
}

func nodeUsageValuesFromMetricsAPI(obj map[string]any) usageValues {
	usage, _, _ := unstructured.NestedStringMap(obj, "usage")
	return usageValuesFromQuantityMap(usage)
}

func usageValuesFromQuantityMap(usage map[string]string) usageValues {
	obj := make(map[string]any, len(usage))
	for key, value := range usage {
		obj[key] = value
	}
	return usageValuesFromAnyQuantityMap(obj)
}

func usageValuesFromAnyQuantityMap(usage map[string]any) usageValues {
	out := usageValues{}
	if raw, ok := usage["cpu"].(string); ok {
		if quantity, err := resource.ParseQuantity(raw); err == nil {
			out.cpuMilli = quantity.MilliValue()
		}
	}
	if raw, ok := usage["memory"].(string); ok {
		if quantity, err := resource.ParseQuantity(raw); err == nil {
			out.memoryBytes = quantity.Value()
		}
	}
	return out
}

func podNodeFromStore(store cache.Store, namespace, pod string) string {
	if store == nil {
		return ""
	}
	obj, exists, err := store.GetByKey(namespace + "/" + pod)
	if err != nil || !exists {
		return ""
	}
	item, _ := obj.(*corev1.Pod)
	if item == nil {
		return ""
	}
	return item.Spec.NodeName
}

func podMetricKey(namespace, pod string) string { return namespace + "/" + pod }
func containerMetricKey(namespace, pod, container string) string {
	return namespace + "/" + pod + "/" + container
}

func splitPodMetricKey(key string) (string, string) {
	ns, pod, _ := strings.Cut(key, "/")
	return ns, pod
}

func (d *RestartDiagnostics) capture(ctx context.Context, incident restartIncident) error {
	workload := d.client.resolvePodWorkload(ctx, incident.pod)
	storage, storageRefs, storageWarnings := d.client.collectRestartDiagnosticStorage(ctx, incident.pod)
	events, storageEvents, eventsErr := d.client.collectRestartDiagnosticEvents(ctx, incident, storageRefs, d.opts.MaxEvents)
	logs := d.client.collectPreviousContainerLogs(ctx, incident.pod, d.opts)
	snapshot := d.buildSnapshot(incident, workload, logs, events)
	snapshot.PersistentStorage = storage
	snapshot.StorageEvents = storageEvents
	snapshot.Warnings = append(snapshot.Warnings, storageWarnings...)
	if eventsErr != nil {
		snapshot.Warnings = append(snapshot.Warnings, "Kubernetes events unavailable")
	}
	if nodeResources, err := d.collectNodeResources(ctx, incident); err == nil {
		snapshot.NodeResources = nodeResources
	} else {
		snapshot.Warnings = append(snapshot.Warnings, "Node capacity unavailable")
	}
	if snapshot.Classification == "application-error" && hasProbeFailure(events) {
		snapshot.Classification = "probe-restart"
	}
	if err := d.client.upsertRestartDiagnosticSnapshot(ctx, snapshot); err != nil {
		return err
	}
	return d.client.cleanupIrrelevantRestartDiagnosticSecrets(ctx, snapshot.Pod.Namespace, restartDiagnosticSecretName(snapshot.Pod))
}

func (d *RestartDiagnostics) buildSnapshot(incident restartIncident, workload RestartDiagnosticWorkload, logs []RestartDiagnosticLog, events []RestartDiagnosticEvent) RestartDiagnosticSnapshot {
	reason := ""
	message := ""
	var exitCode *int32
	if incident.terminated != nil {
		reason = incident.terminated.Reason
		message = incident.terminated.Message
		code := incident.terminated.ExitCode
		exitCode = &code
	}
	if incident.evicted {
		reason = "Evicted"
		message = incident.pod.Status.Message
	}
	resources := containerResources(incident.pod, incident.container, incident.init)
	snapshot := RestartDiagnosticSnapshot{
		Version: restartDiagnosticVersion, IncidentTime: incident.at.UTC(), Classification: classifyRestartIncident(incident),
		Reason: reason, Message: message, ExitCode: exitCode, Workload: workload,
		Pod:          RestartDiagnosticPod{Namespace: incident.pod.Namespace, Name: incident.pod.Name, UID: incident.pod.UID, Node: incident.pod.Spec.NodeName, RestartCount: podRestartCount(incident.pod)},
		Container:    RestartDiagnosticContainer{Name: incident.container, Init: incident.init, RestartCount: incident.restartCount, Resources: resources},
		PreviousLogs: logs, Events: events,
	}
	containerKey := containerMetricKey(incident.pod.Namespace, incident.pod.Name, incident.container)
	for _, offset := range restartDiagnosticOffsets {
		target := incident.at.Add(offset)
		snapshot.ContainerMetrics = append(snapshot.ContainerMetrics, d.metricPoint(target, incident.at, offset, containerKey, incident.pod.Spec.NodeName, true))
		snapshot.NodeMetrics = append(snapshot.NodeMetrics, d.metricPoint(target, incident.at, offset, containerKey, incident.pod.Spec.NodeName, false))
	}
	if sample, ok := d.ring.closest(incident.at, incident.at, d.metricTolerance()); ok {
		snapshot.NodePods = nodePodsFromSample(sample, incident)
	}
	if len(snapshot.ContainerMetrics) > 0 && !snapshot.ContainerMetrics[0].Available {
		snapshot.Warnings = append(snapshot.Warnings, "Metrics history was still warming up; older points are unavailable")
	}
	return snapshot
}

func podRestartCount(pod *corev1.Pod) int32 {
	var total int32
	for _, status := range pod.Status.ContainerStatuses {
		total += status.RestartCount
	}
	for _, status := range pod.Status.InitContainerStatuses {
		total += status.RestartCount
	}
	return total
}

func (d *RestartDiagnostics) metricPoint(target, incidentTime time.Time, offset time.Duration, containerKey, node string, container bool) RestartDiagnosticMetricPoint {
	point := RestartDiagnosticMetricPoint{OffsetSeconds: int64(offset.Seconds()), TargetTime: target.UTC()}
	sample, ok := d.ring.closest(target, incidentTime, d.metricTolerance())
	if !ok {
		return point
	}
	var usage usageValues
	if container {
		usage, ok = sample.Containers[containerKey]
	} else {
		usage, ok = sample.Nodes[node]
	}
	if !ok {
		return point
	}
	point.Available = true
	point.SampledAt = sample.At.UTC()
	point.CPUUsedMilli = usage.cpuMilli
	point.MemoryBytes = usage.memoryBytes
	point.Source = sample.Source
	return point
}

func (d *RestartDiagnostics) metricTolerance() time.Duration {
	tolerance := 2 * d.opts.Interval
	if tolerance < 20*time.Second {
		return 20 * time.Second
	}
	return tolerance
}

func (d *RestartDiagnostics) collectNodeResources(ctx context.Context, incident restartIncident) (RestartDiagnosticNodeResources, error) {
	node, err := d.client.core.CoreV1().Nodes().Get(ctx, incident.pod.Spec.NodeName, metav1.GetOptions{})
	if err != nil {
		return RestartDiagnosticNodeResources{}, err
	}
	out := RestartDiagnosticNodeResources{
		CPUAllocatableMilli:    node.Status.Allocatable.Cpu().MilliValue(),
		CPUCapacityMilli:       node.Status.Capacity.Cpu().MilliValue(),
		MemoryAllocatableBytes: node.Status.Allocatable.Memory().Value(),
		MemoryCapacityBytes:    node.Status.Capacity.Memory().Value(),
	}
	if sample, ok := d.ring.closest(incident.at, incident.at, d.metricTolerance()); ok {
		if usage, available := sample.Nodes[incident.pod.Spec.NodeName]; available {
			out.MetricsAvailable = true
			out.SampledAt = sample.At.UTC()
			out.Source = sample.Source
			out.CPUUsedMilli = usage.cpuMilli
			out.MemoryUsedBytes = usage.memoryBytes
		}
	}
	return out, nil
}

func nodePodsFromSample(sample restartMetricSample, incident restartIncident) []RestartDiagnosticNodePod {
	out := make([]RestartDiagnosticNodePod, 0)
	for _, pod := range sample.Pods {
		if pod.Node != incident.pod.Spec.NodeName {
			continue
		}
		out = append(out, RestartDiagnosticNodePod{
			Namespace: pod.Namespace, Pod: pod.Pod, CPUUsedMilli: pod.Usage.cpuMilli, MemoryUsedBytes: pod.Usage.memoryBytes,
			Affected: pod.Namespace == incident.pod.Namespace && pod.Pod == incident.pod.Name,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].MemoryUsedBytes != out[j].MemoryUsedBytes {
			return out[i].MemoryUsedBytes > out[j].MemoryUsedBytes
		}
		if out[i].Namespace != out[j].Namespace {
			return out[i].Namespace < out[j].Namespace
		}
		return out[i].Pod < out[j].Pod
	})
	if len(out) <= restartDiagnosticNodePodLimit {
		return out
	}
	affected := -1
	for i := range out {
		if out[i].Affected {
			affected = i
			break
		}
	}
	trimmed := append([]RestartDiagnosticNodePod(nil), out[:restartDiagnosticNodePodLimit]...)
	if affected >= restartDiagnosticNodePodLimit {
		trimmed[len(trimmed)-1] = out[affected]
	}
	return trimmed
}

func classifyRestartIncident(incident restartIncident) string {
	if incident.evicted {
		return "eviction"
	}
	if incident.terminated == nil {
		return "termination"
	}
	if strings.EqualFold(incident.terminated.Reason, "OOMKilled") {
		return "oom-killed"
	}
	if strings.EqualFold(incident.terminated.Reason, "Error") || incident.terminated.ExitCode != 0 {
		return "application-error"
	}
	return "termination"
}

func containerResources(pod *corev1.Pod, name string, init bool) RestartDiagnosticResources {
	containers := pod.Spec.Containers
	if init {
		containers = pod.Spec.InitContainers
	}
	for _, container := range containers {
		if container.Name != name {
			continue
		}
		return RestartDiagnosticResources{
			CPURequestMilli: container.Resources.Requests.Cpu().MilliValue(), CPULimitMilli: container.Resources.Limits.Cpu().MilliValue(),
			MemoryRequestBytes: container.Resources.Requests.Memory().Value(), MemoryLimitBytes: container.Resources.Limits.Memory().Value(),
		}
	}
	return RestartDiagnosticResources{}
}

func (c *Client) collectPreviousContainerLogs(ctx context.Context, pod *corev1.Pod, opts RestartDiagnosticsOptions) []RestartDiagnosticLog {
	containers := make([]struct {
		name string
		init bool
	}, 0, len(pod.Spec.InitContainers)+len(pod.Spec.Containers))
	for _, container := range pod.Spec.InitContainers {
		containers = append(containers, struct {
			name string
			init bool
		}{container.Name, true})
	}
	for _, container := range pod.Spec.Containers {
		containers = append(containers, struct {
			name string
			init bool
		}{container.Name, false})
	}
	remaining := opts.MaxTotalLogBytes
	out := make([]RestartDiagnosticLog, 0, len(containers))
	for _, container := range containers {
		entry := RestartDiagnosticLog{Container: container.name, Init: container.init}
		if remaining <= 0 {
			entry.Error = "total log size limit reached"
			out = append(out, entry)
			continue
		}
		tail := opts.MaxLogLines
		raw, err := c.core.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
			Container: container.name, Previous: true, Timestamps: true, TailLines: &tail,
		}).DoRaw(ctx)
		if err != nil {
			entry.Error = "previous logs unavailable"
			out = append(out, entry)
			continue
		}
		limit := opts.MaxLogBytesPerContainer
		if remaining < limit {
			limit = remaining
		}
		if int64(len(raw)) > limit {
			raw = raw[len(raw)-int(limit):]
			entry.Truncated = true
		}
		entry.Available = true
		entry.Text = string(raw)
		remaining -= int64(len(raw))
		out = append(out, entry)
	}
	return out
}

type restartDiagnosticObjectRef struct {
	Kind      string
	Namespace string
	Name      string
	UID       types.UID
}

func (c *Client) collectRestartDiagnosticStorage(ctx context.Context, pod *corev1.Pod) ([]RestartDiagnosticPersistentStorage, []restartDiagnosticObjectRef, []string) {
	storage := make([]RestartDiagnosticPersistentStorage, 0)
	refs := make([]restartDiagnosticObjectRef, 0)
	warnings := make([]string, 0)
	for _, volume := range pod.Spec.Volumes {
		if volume.PersistentVolumeClaim == nil {
			continue
		}
		item := RestartDiagnosticPersistentStorage{
			Volume: volume.Name, ClaimNamespace: pod.Namespace, ClaimName: volume.PersistentVolumeClaim.ClaimName,
		}
		pvc, err := c.core.CoreV1().PersistentVolumeClaims(pod.Namespace).Get(ctx, item.ClaimName, metav1.GetOptions{})
		if err != nil {
			item.Error = "PersistentVolumeClaim unavailable"
			warnings = append(warnings, fmt.Sprintf("PersistentVolumeClaim %s/%s unavailable", pod.Namespace, item.ClaimName))
			storage = append(storage, item)
			refs = append(refs, restartDiagnosticObjectRef{Kind: "PersistentVolumeClaim", Namespace: pod.Namespace, Name: item.ClaimName})
			continue
		}
		refs = append(refs, restartDiagnosticObjectRef{Kind: "PersistentVolumeClaim", Namespace: pvc.Namespace, Name: pvc.Name, UID: pvc.UID})
		item.ClaimStatus = string(pvc.Status.Phase)
		item.RequestedBytes = pvc.Spec.Resources.Requests.Storage().Value()
		item.CapacityBytes = pvc.Status.Capacity.Storage().Value()
		item.VolumeName = pvc.Spec.VolumeName
		if pvc.Spec.StorageClassName != nil {
			item.StorageClass = *pvc.Spec.StorageClassName
		}
		for _, mode := range pvc.Spec.AccessModes {
			item.AccessModes = append(item.AccessModes, string(mode))
		}
		if pvc.Spec.VolumeMode != nil {
			item.VolumeMode = string(*pvc.Spec.VolumeMode)
		}
		if item.VolumeName != "" {
			pv, pvErr := c.core.CoreV1().PersistentVolumes().Get(ctx, item.VolumeName, metav1.GetOptions{})
			if pvErr != nil {
				item.Error = "PersistentVolume unavailable"
				warnings = append(warnings, fmt.Sprintf("PersistentVolume %s unavailable", item.VolumeName))
				refs = append(refs, restartDiagnosticObjectRef{Kind: "PersistentVolume", Name: item.VolumeName})
			} else {
				item.VolumeStatus = string(pv.Status.Phase)
				if item.CapacityBytes == 0 {
					item.CapacityBytes = pv.Spec.Capacity.Storage().Value()
				}
				if item.StorageClass == "" {
					item.StorageClass = pv.Spec.StorageClassName
				}
				refs = append(refs, restartDiagnosticObjectRef{Kind: "PersistentVolume", Name: pv.Name, UID: pv.UID})
			}
		}
		storage = append(storage, item)
	}
	return storage, refs, warnings
}

func (c *Client) collectRestartDiagnosticEvents(ctx context.Context, incident restartIncident, storageRefs []restartDiagnosticObjectRef, maxEvents int) ([]RestartDiagnosticEvent, []RestartDiagnosticEvent, error) {
	eventList, err := c.core.CoreV1().Events("").List(ctx, metav1.ListOptions{})
	if err != nil {
		eventList, err = c.core.CoreV1().Events(incident.pod.Namespace).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return nil, nil, err
	}
	windowStart := incident.at.Add(-10 * time.Minute)
	windowEnd := incident.at.Add(2 * time.Minute)
	out := make([]RestartDiagnosticEvent, 0)
	storageOut := make([]RestartDiagnosticEvent, 0)
	for _, event := range eventList.Items {
		eventTime := kubernetesEventTime(event)
		if eventTime.Before(windowStart) || eventTime.After(windowEnd) {
			continue
		}
		podRelated := event.InvolvedObject.UID == incident.pod.UID ||
			(strings.EqualFold(event.InvolvedObject.Kind, "Pod") && event.InvolvedObject.Name == incident.pod.Name)
		nodeRelated := strings.EqualFold(event.InvolvedObject.Kind, "Node") && event.InvolvedObject.Name == incident.pod.Spec.NodeName && isRelevantNodeEvent(event)
		storageRelated := false
		for _, ref := range storageRefs {
			if eventMatchesDiagnosticRef(event, ref) {
				storageRelated = true
				break
			}
		}
		if !podRelated && !nodeRelated && !storageRelated {
			continue
		}
		entry := RestartDiagnosticEvent{
			Time: eventTime.UTC(), Type: event.Type, Reason: event.Reason,
			Object: event.InvolvedObject.Kind + "/" + event.InvolvedObject.Name, Message: event.Message, Count: event.Count,
		}
		if storageRelated {
			storageOut = append(storageOut, entry)
		} else {
			out = append(out, entry)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time.Before(out[j].Time) })
	sort.Slice(storageOut, func(i, j int) bool { return storageOut[i].Time.Before(storageOut[j].Time) })
	if len(out) > maxEvents {
		out = out[len(out)-maxEvents:]
	}
	if len(storageOut) > maxEvents {
		storageOut = storageOut[len(storageOut)-maxEvents:]
	}
	return out, storageOut, nil
}

func eventMatchesDiagnosticRef(event corev1.Event, ref restartDiagnosticObjectRef) bool {
	if ref.UID != "" && event.InvolvedObject.UID == ref.UID {
		return true
	}
	return strings.EqualFold(event.InvolvedObject.Kind, ref.Kind) && event.InvolvedObject.Name == ref.Name &&
		(ref.Namespace == "" || event.InvolvedObject.Namespace == "" || event.InvolvedObject.Namespace == ref.Namespace)
}

func kubernetesEventTime(event corev1.Event) time.Time {
	if !event.EventTime.IsZero() {
		return event.EventTime.Time
	}
	if !event.LastTimestamp.IsZero() {
		return event.LastTimestamp.Time
	}
	if !event.FirstTimestamp.IsZero() {
		return event.FirstTimestamp.Time
	}
	return event.CreationTimestamp.Time
}

func isRelevantNodeEvent(event corev1.Event) bool {
	text := strings.ToLower(event.Reason + " " + event.Message)
	for _, marker := range []string{"pressure", "notready", "not ready", "reboot", "shutdown", "unreachable", "memory", "disk", "pid"} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func hasProbeFailure(events []RestartDiagnosticEvent) bool {
	for _, event := range events {
		text := strings.ToLower(event.Reason + " " + event.Message)
		if strings.Contains(text, "unhealthy") && (strings.Contains(text, "liveness") || strings.Contains(text, "startup")) {
			return true
		}
	}
	return false
}

func (c *Client) resolvePodWorkload(ctx context.Context, pod *corev1.Pod) RestartDiagnosticWorkload {
	fallback := RestartDiagnosticWorkload{APIVersion: "v1", Kind: "Pod", Namespace: pod.Namespace, Name: pod.Name, UID: pod.UID}
	owner := controllerOwner(pod.OwnerReferences)
	if owner == nil {
		return fallback
	}
	workload := RestartDiagnosticWorkload{APIVersion: owner.APIVersion, Kind: owner.Kind, Namespace: pod.Namespace, Name: owner.Name, UID: owner.UID}
	switch owner.Kind {
	case "ReplicaSet":
		obj, err := c.core.AppsV1().ReplicaSets(pod.Namespace).Get(ctx, owner.Name, metav1.GetOptions{})
		if err != nil {
			return workload
		}
		if parent := controllerOwner(obj.OwnerReferences); parent != nil && parent.Kind == "Deployment" {
			return RestartDiagnosticWorkload{APIVersion: parent.APIVersion, Kind: parent.Kind, Namespace: pod.Namespace, Name: parent.Name, UID: parent.UID}
		}
	case "Job":
		obj, err := c.core.BatchV1().Jobs(pod.Namespace).Get(ctx, owner.Name, metav1.GetOptions{})
		if err != nil {
			return workload
		}
		if parent := controllerOwner(obj.OwnerReferences); parent != nil && parent.Kind == "CronJob" {
			return RestartDiagnosticWorkload{APIVersion: parent.APIVersion, Kind: parent.Kind, Namespace: pod.Namespace, Name: parent.Name, UID: parent.UID}
		}
	}
	return workload
}

func controllerOwner(owners []metav1.OwnerReference) *metav1.OwnerReference {
	for i := range owners {
		if owners[i].Controller != nil && *owners[i].Controller {
			owner := owners[i]
			return &owner
		}
	}
	if len(owners) > 0 {
		owner := owners[0]
		return &owner
	}
	return nil
}

func (c *Client) upsertRestartDiagnosticSnapshot(ctx context.Context, snapshot RestartDiagnosticSnapshot) error {
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	name := restartDiagnosticSecretName(snapshot.Pod)
	secretClient := c.core.CoreV1().Secrets(snapshot.Pod.Namespace)
	secret, err := secretClient.Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		secret = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: snapshot.Pod.Namespace}}
		applyRestartDiagnosticSecretMetadata(secret, snapshot)
		secret.Type = corev1.SecretType("beaverdeck.io/restart-diagnostic")
		secret.Data = map[string][]byte{restartDiagnosticSecretKey: raw}
		if _, err = secretClient.Create(ctx, secret, metav1.CreateOptions{}); err != nil {
			return err
		}
		return c.cleanupRestartDiagnosticSecretsForPod(ctx, snapshot.Pod, name)
	}
	if err != nil {
		return err
	}
	var current RestartDiagnosticSnapshot
	if json.Unmarshal(secret.Data[restartDiagnosticSecretKey], &current) == nil &&
		current.Version == restartDiagnosticVersion && current.Pod.UID == snapshot.Pod.UID && current.IncidentTime.After(snapshot.IncidentTime) {
		return c.cleanupRestartDiagnosticSecretsForPod(ctx, snapshot.Pod, name)
	}
	applyRestartDiagnosticSecretMetadata(secret, snapshot)
	secret.Type = corev1.SecretType("beaverdeck.io/restart-diagnostic")
	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}
	secret.Data[restartDiagnosticSecretKey] = raw
	if _, err = secretClient.Update(ctx, secret, metav1.UpdateOptions{}); err != nil {
		return err
	}
	return c.cleanupRestartDiagnosticSecretsForPod(ctx, snapshot.Pod, name)
}

func applyRestartDiagnosticSecretMetadata(secret *corev1.Secret, snapshot RestartDiagnosticSnapshot) {
	secret.Labels = map[string]string{
		"app.kubernetes.io/managed-by": "beaverdeck",
		"app.kubernetes.io/component":  "restart-diagnostics",
		"beaverdeck.io/secret-purpose": restartDiagnosticSecretPurpose,
	}
	secret.Annotations = map[string]string{
		"beaverdeck.io/workload-kind": snapshot.Workload.Kind,
		"beaverdeck.io/workload-name": snapshot.Workload.Name,
		"beaverdeck.io/container":     snapshot.Container.Name,
		"beaverdeck.io/pod":           snapshot.Pod.Name,
		"beaverdeck.io/pod-uid":       string(snapshot.Pod.UID),
		"beaverdeck.io/incident-time": snapshot.IncidentTime.Format(time.RFC3339Nano),
	}
	secret.OwnerReferences = nil
	if snapshot.Workload.UID != "" && snapshot.Workload.Kind != "Pod" {
		secret.OwnerReferences = []metav1.OwnerReference{{
			APIVersion: snapshot.Workload.APIVersion, Kind: snapshot.Workload.Kind, Name: snapshot.Workload.Name, UID: snapshot.Workload.UID,
		}}
	}
}

func restartDiagnosticSecretName(pod RestartDiagnosticPod) string {
	prefix := strings.Trim(strings.ToLower(strings.TrimSpace(pod.Name)), ".-")
	if prefix == "" {
		prefix = "pod"
	}
	const secretPrefix = "beaverdeck-restart-"
	if len(prefix) > 253-len(secretPrefix) {
		prefix = strings.Trim(prefix[:253-len(secretPrefix)], "-")
	}
	return secretPrefix + prefix
}

func (c *Client) ListRestartDiagnosticSummaries(ctx context.Context, namespace string) ([]RestartDiagnosticSummary, error) {
	secrets, err := c.core.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{LabelSelector: "beaverdeck.io/secret-purpose=" + restartDiagnosticSecretPurpose})
	if err != nil {
		return nil, err
	}
	latestByPod := make(map[string]RestartDiagnosticSummary, len(secrets.Items))
	for _, secret := range secrets.Items {
		var snapshot RestartDiagnosticSnapshot
		if err := json.Unmarshal(secret.Data[restartDiagnosticSecretKey], &snapshot); err != nil || snapshot.Version != restartDiagnosticVersion {
			continue
		}
		normalizeRestartDiagnosticSnapshot(&snapshot)
		summary := RestartDiagnosticSummary{
			SecretName: secret.Name, IncidentTime: snapshot.IncidentTime, Classification: snapshot.Classification, Reason: snapshot.Reason,
			Workload: snapshot.Workload, Pod: snapshot.Pod, Container: snapshot.Container.Name, RestartCount: snapshot.Pod.RestartCount,
		}
		key := snapshot.Pod.Namespace + "/" + snapshot.Pod.Name
		if current, ok := latestByPod[key]; !ok || current.IncidentTime.Before(summary.IncidentTime) {
			latestByPod[key] = summary
		}
	}
	out := make([]RestartDiagnosticSummary, 0, len(latestByPod))
	for _, summary := range latestByPod {
		out = append(out, summary)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].IncidentTime.After(out[j].IncidentTime) })
	return out, nil
}

func (c *Client) cleanupRestartDiagnosticSecretsForPod(ctx context.Context, pod RestartDiagnosticPod, keep string) error {
	secrets, err := c.core.CoreV1().Secrets(pod.Namespace).List(ctx, metav1.ListOptions{LabelSelector: "beaverdeck.io/secret-purpose=" + restartDiagnosticSecretPurpose})
	if err != nil {
		return err
	}
	for _, secret := range secrets.Items {
		if secret.Name == keep {
			continue
		}
		samePod := secret.Annotations["beaverdeck.io/pod"] == pod.Name
		if !samePod {
			var snapshot RestartDiagnosticSnapshot
			if json.Unmarshal(secret.Data[restartDiagnosticSecretKey], &snapshot) == nil {
				samePod = snapshot.Pod.Namespace == pod.Namespace && snapshot.Pod.Name == pod.Name
			}
		}
		if samePod {
			if err := c.core.CoreV1().Secrets(pod.Namespace).Delete(ctx, secret.Name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}
	}
	return nil
}

func (c *Client) cleanupIrrelevantRestartDiagnosticSecrets(ctx context.Context, namespace, keep string) error {
	pods, err := c.core.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	livePods := make(map[string]types.UID, len(pods.Items))
	for _, pod := range pods.Items {
		livePods[pod.Name] = pod.UID
	}

	secrets, err := c.core.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{LabelSelector: "beaverdeck.io/secret-purpose=" + restartDiagnosticSecretPurpose})
	if err != nil {
		return err
	}
	for _, secret := range secrets.Items {
		if secret.Name == keep {
			continue
		}
		var snapshot RestartDiagnosticSnapshot
		if json.Unmarshal(secret.Data[restartDiagnosticSecretKey], &snapshot) != nil || snapshot.Version != restartDiagnosticVersion || snapshot.Pod.Name == "" {
			continue
		}
		liveUID, exists := livePods[snapshot.Pod.Name]
		if exists && (snapshot.Pod.UID == "" || snapshot.Pod.UID == liveUID) {
			continue
		}
		if err := c.core.CoreV1().Secrets(namespace).Delete(ctx, secret.Name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func (c *Client) migrateRestartDiagnosticSecrets(ctx context.Context, namespace string) error {
	secrets, err := c.core.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{LabelSelector: "beaverdeck.io/secret-purpose=" + restartDiagnosticSecretPurpose})
	if err != nil {
		return err
	}
	latest := make(map[string]RestartDiagnosticSnapshot)
	for _, secret := range secrets.Items {
		var snapshot RestartDiagnosticSnapshot
		if json.Unmarshal(secret.Data[restartDiagnosticSecretKey], &snapshot) != nil || snapshot.Version != restartDiagnosticVersion || snapshot.Pod.Name == "" {
			continue
		}
		normalizeRestartDiagnosticSnapshot(&snapshot)
		key := snapshot.Pod.Namespace + "/" + snapshot.Pod.Name
		if current, ok := latest[key]; !ok || current.IncidentTime.Before(snapshot.IncidentTime) {
			latest[key] = snapshot
		}
	}
	for _, snapshot := range latest {
		if err := c.upsertRestartDiagnosticSnapshot(ctx, snapshot); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) GetRestartDiagnosticSnapshot(ctx context.Context, namespace, name string) (RestartDiagnosticSnapshot, error) {
	secret, err := c.core.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return RestartDiagnosticSnapshot{}, err
	}
	if secret.Labels["beaverdeck.io/secret-purpose"] != restartDiagnosticSecretPurpose {
		return RestartDiagnosticSnapshot{}, fmt.Errorf("secret is not a restart diagnostic")
	}
	var snapshot RestartDiagnosticSnapshot
	if err := json.Unmarshal(secret.Data[restartDiagnosticSecretKey], &snapshot); err != nil {
		return RestartDiagnosticSnapshot{}, err
	}
	if snapshot.Version != restartDiagnosticVersion {
		return RestartDiagnosticSnapshot{}, fmt.Errorf("unsupported restart diagnostic version %d", snapshot.Version)
	}
	normalizeRestartDiagnosticSnapshot(&snapshot)
	return snapshot, nil
}

func normalizeRestartDiagnosticSnapshot(snapshot *RestartDiagnosticSnapshot) {
	if snapshot.Pod.RestartCount == 0 && snapshot.Container.RestartCount > 0 {
		snapshot.Pod.RestartCount = snapshot.Container.RestartCount
	}
}
