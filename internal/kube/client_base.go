package kube

import (
	"fmt"
	"sync"
	"time"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
)

type Client struct {
	core      kubernetes.Interface
	dyn       dynamic.Interface
	rest      *rest.Config
	mapper    *restmapper.DeferredDiscoveryRESTMapper
	discovery discovery.CachedDiscoveryInterface
	metrics   resourceMetricsCache
	dcgm      dcgmMetricsCache
}

type Workload struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Ready     string `json:"ready"`
	Age       string `json:"age"`
}

type ClusterRoleInfo struct {
	Name  string `json:"name"`
	Rules int    `json:"rules"`
	Age   string `json:"age"`
}

type ClusterRoleBindingInfo struct {
	Name     string `json:"name"`
	RoleRef  string `json:"role_ref"`
	Subjects string `json:"subjects"`
	Age      string `json:"age"`
}

type RoleInfo struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Rules     int    `json:"rules"`
	Age       string `json:"age"`
}

type RoleBindingInfo struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	RoleRef   string `json:"role_ref"`
	Subjects  string `json:"subjects"`
	Age       string `json:"age"`
}

type ServiceAccountInfo struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Secrets   int    `json:"secrets"`
	Age       string `json:"age"`
}

type PodInfo struct {
	Namespace           string   `json:"namespace"`
	Name                string   `json:"name"`
	Phase               string   `json:"phase"`
	Ready               string   `json:"ready"`
	Restarts            int32    `json:"restarts"`
	Age                 string   `json:"age"`
	Node                string   `json:"node"`
	Containers          []string `json:"containers"`
	MetricsAvailable    bool     `json:"metrics_available"`
	CPU                 string   `json:"cpu"`
	CPUUsedMilli        int64    `json:"cpu_used_milli"`
	CPURequestMilli     int64    `json:"cpu_request_milli"`
	CPULimitMilli       int64    `json:"cpu_limit_milli"`
	CPUTotalMilli       int64    `json:"cpu_total_milli"`
	Memory              string   `json:"memory"`
	MemoryUsedBytes     int64    `json:"memory_used_bytes"`
	MemoryRequestBytes  int64    `json:"memory_request_bytes"`
	MemoryLimitBytes    int64    `json:"memory_limit_bytes"`
	MemoryTotalBytes    int64    `json:"memory_total_bytes"`
	GPU                 string   `json:"gpu"`
	GPUMetricsAvailable bool     `json:"gpu_metrics_available"`
	GPUUsedPercent      int64    `json:"gpu_used_percent"`
	GPUMemoryUsedBytes  int64    `json:"gpu_memory_used_bytes"`
	GPURequestCount     int64    `json:"gpu_request_count"`
	GPUDeviceCount      int64    `json:"gpu_device_count"`
}

type EventInfo struct {
	Namespace string `json:"namespace"`
	Type      string `json:"type"`
	Reason    string `json:"reason"`
	Object    string `json:"object"`
	Message   string `json:"message"`
	Count     int32  `json:"count"`
	LastSeen  string `json:"last_seen"`
}

type NodeInfo struct {
	Name                string            `json:"name"`
	Status              string            `json:"status"`
	Roles               string            `json:"roles"`
	Age                 string            `json:"age"`
	Labels              map[string]string `json:"labels,omitempty"`
	Pods                string            `json:"pods"`
	PodCount            int64             `json:"pod_count"`
	MaxPodCount         int64             `json:"max_pod_count"`
	MetricsAvailable    bool              `json:"metrics_available"`
	CPU                 string            `json:"cpu"`
	CPUUsedMilli        int64             `json:"cpu_used_milli"`
	CPUTotalMilli       int64             `json:"cpu_total_milli"`
	Memory              string            `json:"memory"`
	MemoryUsedBytes     int64             `json:"memory_used_bytes"`
	MemoryTotalBytes    int64             `json:"memory_total_bytes"`
	HasGPU              bool              `json:"has_gpu"`
	GPUCount            int64             `json:"gpu_count"`
	GPU                 string            `json:"gpu"`
	GPUMetricsAvailable bool              `json:"gpu_metrics_available"`
	GPUUsedPercent      int64             `json:"gpu_used_percent"`
	GPUMemoryUsedBytes  int64             `json:"gpu_memory_used_bytes"`
}

type IngressInfo struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Class     string `json:"class"`
	Hosts     string `json:"hosts"`
	Address   string `json:"address"`
	Age       string `json:"age"`
}

type SecretInfo struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	DataKeys  int    `json:"data_keys"`
	Age       string `json:"age"`
}

type ConfigMapInfo struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	DataKeys  int    `json:"data_keys"`
	Age       string `json:"age"`
}

type CRDInfo struct {
	Name     string `json:"name"`
	Group    string `json:"group"`
	Kind     string `json:"kind"`
	Resource string `json:"resource"`
	Version  string `json:"version"`
	Scope    string `json:"scope"`
	Versions string `json:"versions"`
	Age      string `json:"age"`
}

type CustomResourceDefinition struct {
	Name       string
	Group      string
	Version    string
	Resource   string
	Kind       string
	Namespaced bool
}

type CustomResourceInfo struct {
	Namespace  string `json:"namespace,omitempty"`
	Name       string `json:"name"`
	APIVersion string `json:"api_version"`
	Kind       string `json:"kind"`
	Age        string `json:"age"`
}

type HelmReleaseInfo struct {
	Namespace    string `json:"namespace"`
	Name         string `json:"name"`
	Revision     int    `json:"revision"`
	Status       string `json:"status"`
	Chart        string `json:"chart"`
	ChartVersion string `json:"chart_version"`
	AppVersion   string `json:"app_version"`
	Updated      string `json:"updated"`
	Description  string `json:"description"`
}

type ArgoCDApplicationInfo struct {
	Namespace    string `json:"namespace"`
	Name         string `json:"name"`
	Project      string `json:"project"`
	SyncStatus   string `json:"sync_status"`
	HealthStatus string `json:"health_status"`
	Revision     string `json:"revision"`
	Source       string `json:"source"`
	Destination  string `json:"destination"`
	Updated      string `json:"updated"`
}

type ArgoCDApplicationRevisionInfo struct {
	Namespace     string `json:"namespace"`
	Name          string `json:"name"`
	ID            int64  `json:"id"`
	Revision      string `json:"revision"`
	Source        string `json:"source"`
	DeployStarted string `json:"deploy_started"`
	Deployed      string `json:"deployed"`
	InitiatedBy   string `json:"initiated_by"`
	Current       bool   `json:"current"`
}

type ServiceInfo struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	ClusterIP string `json:"cluster_ip"`
	Ports     string `json:"ports"`
	Age       string `json:"age"`
}

type PVCInfo struct {
	Namespace        string `json:"namespace"`
	Name             string `json:"name"`
	Status           string `json:"status"`
	Volume           string `json:"volume"`
	Capacity         string `json:"capacity"`
	StorageClass     string `json:"storage_class"`
	Usage            string `json:"usage"`
	MetricsAvailable bool   `json:"metrics_available"`
	UsedBytes        int64  `json:"used_bytes"`
	CapacityBytes    int64  `json:"capacity_bytes"`
	Age              string `json:"age"`
}

type PVInfo struct {
	Name             string `json:"name"`
	Status           string `json:"status"`
	Capacity         string `json:"capacity"`
	Claim            string `json:"claim"`
	StorageClass     string `json:"storage_class"`
	Usage            string `json:"usage"`
	MetricsAvailable bool   `json:"metrics_available"`
	UsedBytes        int64  `json:"used_bytes"`
	CapacityBytes    int64  `json:"capacity_bytes"`
	Age              string `json:"age"`
}

type StorageClassInfo struct {
	Name              string `json:"name"`
	Provisioner       string `json:"provisioner"`
	ReclaimPolicy     string `json:"reclaim_policy"`
	VolumeBindingMode string `json:"volume_binding_mode"`
	DefaultClass      bool   `json:"default_class"`
	Age               string `json:"age"`
}

type ApplyResult struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Namespace  string `json:"namespace"`
	Name       string `json:"name"`
}

type NodeDrainResult struct {
	Node     string   `json:"node"`
	Cordoned bool     `json:"cordoned"`
	Evicted  []string `json:"evicted"`
	Skipped  []string `json:"skipped"`
	Failed   []string `json:"failed"`
}

type InsightAlert struct {
	Key          string   `json:"key"`
	CheckType    string   `json:"check_type"`
	CheckLabel   string   `json:"check_label"`
	Status       string   `json:"status"`
	Category     string   `json:"category"`
	Severity     string   `json:"severity"`
	Title        string   `json:"title"`
	Summary      string   `json:"summary"`
	Namespace    string   `json:"namespace,omitempty"`
	ResourceKind string   `json:"resource_kind,omitempty"`
	ResourceName string   `json:"resource_name,omitempty"`
	Node         string   `json:"node,omitempty"`
	Details      []string `json:"details,omitempty"`
	Suppressed   bool     `json:"suppressed"`
}

type pvcVolumeUsage struct {
	UsedBytes      int64
	CapacityBytes  int64
	AvailableBytes int64
}

type summaryStats struct {
	Pods []summaryPod `json:"pods"`
}

type summaryPod struct {
	PodRef  summaryPodRef   `json:"podRef"`
	Volumes []summaryVolume `json:"volume"`
}

type summaryPodRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type summaryVolume struct {
	Name           string          `json:"name"`
	PVCRef         *summaryPVCRef  `json:"pvcRef,omitempty"`
	FS             *summaryFSStats `json:"fsStats,omitempty"`
	AvailableBytes *uint64         `json:"availableBytes,omitempty"`
	CapacityBytes  *uint64         `json:"capacityBytes,omitempty"`
	UsedBytes      *uint64         `json:"usedBytes,omitempty"`
}

type summaryPVCRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type summaryFSStats struct {
	AvailableBytes *uint64 `json:"availableBytes,omitempty"`
	CapacityBytes  *uint64 `json:"capacityBytes,omitempty"`
	UsedBytes      *uint64 `json:"usedBytes,omitempty"`
}

type usageValues struct {
	cpuMilli    int64
	memoryBytes int64
}

type gpuUsageValues struct {
	utilPercent     int64
	memoryUsedBytes int64
	deviceCount     int64
	hasUtil         bool
	hasMemory       bool
}

type dcgmMetricsSnapshot struct {
	available bool
	nodeUsage map[string]gpuUsageValues
	podUsage  map[string]gpuUsageValues
}

type dcgmMetricsCache struct {
	mu        sync.Mutex
	snapshot  dcgmMetricsSnapshot
	expiresAt time.Time
}

type dcgmEntityUsage struct {
	mig         bool
	hasUtil     bool
	utilPercent float64
	hasFBUsed   bool
	fbUsedMiB   float64
}

type dcgmUsageAggregate struct {
	entities map[string]dcgmEntityUsage
}

type resourceCounterSample struct {
	value     float64
	sampledAt time.Time
}

type resourceMetricsCache struct {
	mu      sync.Mutex
	podCPU  map[string]resourceCounterSample
	nodeCPU map[string]resourceCounterSample
}

type resourceMetricsScrape struct {
	resourceScrapeError bool
	podCPUSeconds       map[string]float64
	podMemoryBytes      map[string]int64
	nodeCPUSeconds      float64
	nodeMemoryBytes     int64
	hasNodeCPU          bool
	hasNodeMemory       bool
}

type resourceMetricsStatus struct {
	metricsServerAvailable bool
	directAvailable        bool
}

func InCluster() (*Client, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("in-cluster config: %w", err)
	}

	core, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("kubernetes client: %w", err)
	}
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("dynamic client: %w", err)
	}

	dc, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("discovery client: %w", err)
	}
	cached := memory.NewMemCacheClient(dc)
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(cached)

	return &Client{
		core:      core,
		dyn:       dyn,
		rest:      cfg,
		mapper:    mapper,
		discovery: cached,
		metrics: resourceMetricsCache{
			podCPU:  make(map[string]resourceCounterSample),
			nodeCPU: make(map[string]resourceCounterSample),
		},
	}, nil
}
