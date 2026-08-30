package kube

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
)

func TestDetectRestartIncidentsTransitionAndDedupeInputs(t *testing.T) {
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	oldPod := diagnosticTestPod(now)
	newPod := oldPod.DeepCopy()
	newPod.Status.ContainerStatuses[0].RestartCount = 2
	newPod.Status.ContainerStatuses[0].LastTerminationState.Terminated = &corev1.ContainerStateTerminated{Reason: "Error", ExitCode: 2, FinishedAt: metav1.NewTime(now.Add(-time.Second))}
	newPod.Status.InitContainerStatuses[0].RestartCount = 1
	newPod.Status.InitContainerStatuses[0].LastTerminationState.Terminated = &corev1.ContainerStateTerminated{Reason: "Error", ExitCode: 1, FinishedAt: metav1.NewTime(now.Add(-2 * time.Second))}

	incidents := detectRestartIncidents(oldPod, newPod, false, now)
	if len(incidents) != 1 {
		t.Fatalf("expected one pod incident, got %#v", incidents)
	}
	if incidents[0].container != "app" || incidents[0].init {
		t.Fatalf("unexpected incidents: %#v", incidents)
	}
	if duplicate := detectRestartIncidents(newPod, newPod.DeepCopy(), false, now); len(duplicate) != 0 {
		t.Fatalf("expected duplicate watch update to be ignored, got %#v", duplicate)
	}
	withoutLastState := newPod.DeepCopy()
	withoutLastState.Status.ContainerStatuses[0].RestartCount++
	withoutLastState.Status.ContainerStatuses[0].LastTerminationState.Terminated = nil
	partial := detectRestartIncidents(newPod, withoutLastState, false, now)
	if len(partial) != 1 || partial[0].terminated != nil {
		t.Fatalf("restart count transition must survive missing lastState: %#v", partial)
	}

	d := &RestartDiagnostics{queue: make(chan restartIncident, 4), dedupe: map[string]struct{}{}}
	d.enqueue(incidents[0])
	d.enqueue(incidents[0])
	if len(d.queue) != 1 {
		t.Fatalf("expected deduplicated queue length 1, got %d", len(d.queue))
	}
}

func TestDetectRestartIncidentsDeletionAndEviction(t *testing.T) {
	now := time.Now().UTC()
	pod := diagnosticTestPod(now)
	if incidents := detectRestartIncidents(nil, pod, true, now); len(incidents) != 0 {
		t.Fatalf("normal deletion must not create incident: %#v", incidents)
	}
	replacement := pod.DeepCopy()
	replacement.UID = typesUID("replacement-uid")
	replacement.Status.ContainerStatuses[0].RestartCount = 0
	if incidents := detectRestartIncidents(nil, replacement, false, now); len(incidents) != 0 {
		t.Fatalf("normal replacement pod must not create incident: %#v", incidents)
	}

	evicted := pod.DeepCopy()
	evicted.Status.Phase = corev1.PodFailed
	evicted.Status.Reason = "Evicted"
	evicted.Status.Message = "node was low on memory"
	incidents := detectRestartIncidents(pod, evicted, false, now)
	if len(incidents) != 1 || !incidents[0].evicted {
		t.Fatalf("expected one pod eviction incident, got %#v", incidents)
	}
}

func TestRestartMetricRingLookupAndPartialHistory(t *testing.T) {
	base := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	ring := newRestartMetricRing(5*time.Minute, 10*time.Second)
	ring.add(restartMetricSample{At: base.Add(-40 * time.Second), Source: "old"})
	ring.add(restartMetricSample{At: base.Add(-20 * time.Second), Source: "new"})

	sample, ok := ring.closestAtOrBefore(base.Add(-25 * time.Second))
	if !ok || sample.Source != "old" {
		t.Fatalf("expected closest preceding sample, got %#v, %v", sample, ok)
	}
	if _, ok := ring.closestAtOrBefore(base.Add(-time.Minute)); ok {
		t.Fatal("expected unavailable partial history before the first sample")
	}

	d := &RestartDiagnostics{ring: ring, opts: RestartDiagnosticsOptions{Interval: 10 * time.Second}}
	point := d.metricPoint(base.Add(-time.Minute), base, -time.Minute, "apps/pod/app", "node-a", true)
	if point.Available {
		t.Fatalf("expected explicit unavailable point, got %#v", point)
	}
	ring.add(restartMetricSample{At: base.Add(-58 * time.Second), Source: "jittered", Containers: map[string]usageValues{"apps/pod/app": {cpuMilli: 25}}})
	point = d.metricPoint(base.Add(-time.Minute), base, -time.Minute, "apps/pod/app", "node-a", true)
	if !point.Available || point.CPUUsedMilli != 25 || point.Source != "jittered" {
		t.Fatalf("expected closest sample within interval jitter, got %#v", point)
	}
}

func TestRestartMetricPointUsesMetricsAPITimestampObservedAfterIncident(t *testing.T) {
	incidentAt := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	metricAt := incidentAt.Add(-10 * time.Second)
	ring := newRestartMetricRing(5*time.Minute, 10*time.Second)
	ring.add(restartMetricSample{
		At:             incidentAt.Add(2 * time.Second),
		Source:         "metrics-server",
		Containers:     map[string]usageValues{"apps/pod/app": {cpuMilli: 40, memoryBytes: 4096}},
		ContainerTimes: map[string]time.Time{"apps/pod/app": metricAt},
		Pods: map[string]diagnosticPodMetric{
			"apps/pod": {Namespace: "apps", Pod: "pod", Node: "node-a", SampledAt: metricAt, Usage: usageValues{cpuMilli: 40, memoryBytes: 4096}},
		},
	})
	diagnostics := &RestartDiagnostics{ring: ring, opts: RestartDiagnosticsOptions{Interval: 10 * time.Second}}
	point := diagnostics.metricPoint(metricAt, incidentAt, -10*time.Second, "apps/pod/app", "node-a", true)
	if !point.Available || !point.SampledAt.Equal(metricAt) || point.CPUUsedMilli != 40 || point.MemoryBytes != 4096 {
		t.Fatalf("expected delayed observation to retain the pre-incident metric timestamp, got %#v", point)
	}
	if _, ok := ring.closestPodSample(incidentAt, incidentAt, 20*time.Second, "apps/pod"); !ok {
		t.Fatal("expected delayed pod observation to remain eligible for the incident node snapshot")
	}
}

func TestRestartDiagnosticMetricOffsetsStartAtThreeMinutes(t *testing.T) {
	want := []time.Duration{-3 * time.Minute, -time.Minute, -30 * time.Second, -10 * time.Second}
	if len(restartDiagnosticOffsets) != len(want) {
		t.Fatalf("unexpected metric offsets: %#v", restartDiagnosticOffsets)
	}
	for i := range want {
		if restartDiagnosticOffsets[i] != want[i] {
			t.Fatalf("metric offset %d: got %s, want %s", i, restartDiagnosticOffsets[i], want[i])
		}
	}
}

func TestCollectRestartDiagnosticMetricsFromMetricsServer(t *testing.T) {
	podGVR := schema.GroupVersionResource{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "pods"}
	nodeGVR := schema.GroupVersionResource{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "nodes"}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
		podGVR: "PodMetricsList", nodeGVR: "NodeMetricsList",
	},
		&unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "metrics.k8s.io/v1beta1", "kind": "Pod",
			"metadata":   map[string]any{"namespace": "apps", "name": "api-1"},
			"timestamp":  "2026-08-17T11:59:50Z",
			"containers": []any{map[string]any{"name": "app", "usage": map[string]any{"cpu": "125m", "memory": "64Mi"}}},
		}},
		&unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "metrics.k8s.io/v1beta1", "kind": "Node",
			"metadata": map[string]any{"name": "node-a"}, "usage": map[string]any{"cpu": "2", "memory": "4Gi"},
		}},
	)
	store := cache.NewStore(cache.MetaNamespaceKeyFunc)
	_ = store.Add(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "apps", Name: "api-1"}, Spec: corev1.PodSpec{NodeName: "node-a"}})
	client := &Client{core: fake.NewSimpleClientset(), dyn: dyn}
	sample := client.collectRestartDiagnosticMetrics(context.Background(), "apps", store, time.Now())
	if sample.Source != "metrics-server" {
		t.Fatalf("expected metrics-server source, got %q", sample.Source)
	}
	usage := sample.Containers["apps/api-1/app"]
	if usage.cpuMilli != 125 || usage.memoryBytes != 64*1024*1024 {
		t.Fatalf("unexpected container usage: %#v (all=%#v)", usage, sample.Containers)
	}
	if sample.Pods["apps/api-1"].Node != "node-a" || sample.Nodes["node-a"].cpuMilli != 2000 {
		t.Fatalf("unexpected pod/node sample: %#v", sample)
	}
	if got := sample.ContainerTimes["apps/api-1/app"]; !got.Equal(time.Date(2026, 8, 17, 11, 59, 50, 0, time.UTC)) {
		t.Fatalf("unexpected container metric timestamp: %s", got)
	}
}

func TestCollectNodeResourcesIncludesUsageAllocatableAndCapacity(t *testing.T) {
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	client := &Client{core: fake.NewSimpleClientset(&corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-a"},
		Status: corev1.NodeStatus{
			Allocatable: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("3500m"), corev1.ResourceMemory: resource.MustParse("7Gi")},
			Capacity:    corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("4"), corev1.ResourceMemory: resource.MustParse("8Gi")},
		},
	})}
	ring := newRestartMetricRing(5*time.Minute, 10*time.Second)
	ring.add(restartMetricSample{At: now.Add(-time.Second), Source: "metrics-server", Nodes: map[string]usageValues{"node-a": {cpuMilli: 1200, memoryBytes: 3 * 1024 * 1024 * 1024}}})
	diagnostics := &RestartDiagnostics{client: client, ring: ring, opts: RestartDiagnosticsOptions{Interval: 10 * time.Second}}
	resources, err := diagnostics.collectNodeResources(context.Background(), restartIncident{pod: diagnosticTestPod(now), at: now})
	if err != nil || !resources.MetricsAvailable || resources.CPUUsedMilli != 1200 || resources.CPUAllocatableMilli != 3500 || resources.CPUCapacityMilli != 4000 || resources.MemoryCapacityBytes != 8*1024*1024*1024 {
		t.Fatalf("unexpected node resources: %#v, %v", resources, err)
	}
}

func TestMetricsUnavailableAndKubeletContainerParsing(t *testing.T) {
	client := &Client{core: fake.NewSimpleClientset()}
	sample := client.collectRestartDiagnosticMetrics(context.Background(), "apps", nil, time.Now())
	if sample.Source != "" || len(sample.Containers) != 0 {
		t.Fatalf("expected graceful unavailable metrics, got %#v", sample)
	}
	raw := []byte(`container_cpu_usage_seconds_total{namespace="apps",pod="api-1",container="app"} 12.5
container_memory_working_set_bytes{namespace="apps",pod="api-1",container="app"} 4096
pod_memory_working_set_bytes{namespace="apps",pod="api-1"} 8192
node_memory_working_set_bytes 16384`)
	scrape, err := parseResourceMetrics(raw)
	if err != nil || scrape.containerCPUSeconds["apps/api-1/app"] != 12.5 || scrape.containerMemoryBytes["apps/api-1/app"] != 4096 {
		t.Fatalf("unexpected kubelet fallback parse: %#v, %v", scrape, err)
	}
}

func TestRestartDiagnosticSnapshotJSONVersionAndSecretOverwrite(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	client := &Client{core: clientset}
	snapshot := diagnosticTestSnapshot(time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC))
	if err := client.upsertRestartDiagnosticSnapshot(context.Background(), snapshot); err != nil {
		t.Fatalf("create snapshot: %v", err)
	}
	snapshot.IncidentTime = snapshot.IncidentTime.Add(time.Minute)
	snapshot.Container.RestartCount = 3
	if err := client.upsertRestartDiagnosticSnapshot(context.Background(), snapshot); err != nil {
		t.Fatalf("overwrite snapshot: %v", err)
	}
	secrets, err := clientset.CoreV1().Secrets("apps").List(context.Background(), metav1.ListOptions{})
	if err != nil || len(secrets.Items) != 1 {
		t.Fatalf("expected one overwritten Secret, got %d, %v", len(secrets.Items), err)
	}
	if secrets.Items[0].Name != "beaverdeck-restart-api-1" {
		t.Fatalf("expected deterministic pod Secret name without hash, got %q", secrets.Items[0].Name)
	}
	var decoded RestartDiagnosticSnapshot
	if err := json.Unmarshal(secrets.Items[0].Data[restartDiagnosticSecretKey], &decoded); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	if decoded.Version != 1 || decoded.Container.RestartCount != 3 {
		t.Fatalf("unexpected snapshot JSON: %#v", decoded)
	}
	if len(secrets.Items[0].OwnerReferences) != 1 || secrets.Items[0].OwnerReferences[0].Kind != "Deployment" {
		t.Fatalf("expected workload owner reference, got %#v", secrets.Items[0].OwnerReferences)
	}
	older := snapshot
	older.IncidentTime = snapshot.IncidentTime.Add(-2 * time.Minute)
	older.Container.RestartCount = 1
	if err := client.upsertRestartDiagnosticSnapshot(context.Background(), older); err != nil {
		t.Fatalf("ignore older snapshot: %v", err)
	}
	stored, err := client.GetRestartDiagnosticSnapshot(context.Background(), "apps", secrets.Items[0].Name)
	if err != nil || stored.Container.RestartCount != 3 {
		t.Fatalf("older worker must not overwrite latest snapshot: %#v, %v", stored, err)
	}
	replacement := older
	replacement.Pod.UID = typesUID("replacement-pod-uid")
	if err := client.upsertRestartDiagnosticSnapshot(context.Background(), replacement); err != nil {
		t.Fatalf("replace snapshot for recreated pod: %v", err)
	}
	stored, err = client.GetRestartDiagnosticSnapshot(context.Background(), "apps", secrets.Items[0].Name)
	if err != nil || stored.Pod.UID != replacement.Pod.UID {
		t.Fatalf("recreated pod must replace same-name old UID snapshot: %#v, %v", stored, err)
	}
}

func TestRestartDiagnosticSecretMigrationKeepsNewestPerPod(t *testing.T) {
	base := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	older := diagnosticTestSnapshot(base)
	newer := diagnosticTestSnapshot(base.Add(time.Minute))
	newer.Container.Name = "sidecar"
	newer.Container.RestartCount = 4
	oldRaw, _ := json.Marshal(older)
	newRaw, _ := json.Marshal(newer)
	labels := map[string]string{"beaverdeck.io/secret-purpose": restartDiagnosticSecretPurpose}
	clientset := fake.NewSimpleClientset(
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "apps", Name: "legacy-app", Labels: labels}, Data: map[string][]byte{restartDiagnosticSecretKey: oldRaw}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "apps", Name: "legacy-sidecar", Labels: labels}, Data: map[string][]byte{restartDiagnosticSecretKey: newRaw}},
	)
	client := &Client{core: clientset}
	if err := client.migrateRestartDiagnosticSecrets(context.Background(), "apps"); err != nil {
		t.Fatalf("migrate diagnostics: %v", err)
	}
	secrets, err := clientset.CoreV1().Secrets("apps").List(context.Background(), metav1.ListOptions{})
	if err != nil || len(secrets.Items) != 1 || secrets.Items[0].Name != "beaverdeck-restart-api-1" {
		t.Fatalf("expected one pod Secret after migration, got %#v, %v", secrets.Items, err)
	}
	var stored RestartDiagnosticSnapshot
	if err := json.Unmarshal(secrets.Items[0].Data[restartDiagnosticSecretKey], &stored); err != nil || stored.Container.Name != "sidecar" {
		t.Fatalf("expected newest snapshot to survive migration, got %#v, %v", stored, err)
	}
}

func TestCleanupIrrelevantRestartDiagnosticSecrets(t *testing.T) {
	makeSecret := func(name, podName string, podUID types.UID) *corev1.Secret {
		snapshot := diagnosticTestSnapshot(time.Now().UTC())
		snapshot.Pod.Name = podName
		snapshot.Pod.UID = podUID
		raw, _ := json.Marshal(snapshot)
		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: "apps", Name: name, Labels: map[string]string{"beaverdeck.io/secret-purpose": restartDiagnosticSecretPurpose}},
			Data:       map[string][]byte{restartDiagnosticSecretKey: raw},
		}
	}
	clientset := fake.NewSimpleClientset(
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "apps", Name: "live", UID: typesUID("live-uid")}},
		makeSecret("keep-current-incident", "evicted-now", typesUID("evicted-uid")),
		makeSecret("keep-live", "live", typesUID("live-uid")),
		makeSecret("delete-missing", "gone", typesUID("gone-uid")),
		makeSecret("delete-recreated", "live", typesUID("old-live-uid")),
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "apps", Name: "keep-malformed", Labels: map[string]string{"beaverdeck.io/secret-purpose": restartDiagnosticSecretPurpose}}, Data: map[string][]byte{restartDiagnosticSecretKey: []byte("not-json")}},
	)
	client := &Client{core: clientset}
	if err := client.cleanupIrrelevantRestartDiagnosticSecrets(context.Background(), "apps", "keep-current-incident"); err != nil {
		t.Fatalf("cleanup irrelevant snapshots: %v", err)
	}
	secrets, err := clientset.CoreV1().Secrets("apps").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("list snapshots after cleanup: %v", err)
	}
	names := make(map[string]bool, len(secrets.Items))
	for _, secret := range secrets.Items {
		names[secret.Name] = true
	}
	for _, name := range []string{"keep-current-incident", "keep-live", "keep-malformed"} {
		if !names[name] {
			t.Fatalf("expected %s to remain, got %#v", name, names)
		}
	}
	for _, name := range []string{"delete-missing", "delete-recreated"} {
		if names[name] {
			t.Fatalf("expected %s to be removed, got %#v", name, names)
		}
	}
}

func TestPreviousLogsCollectAllContainersAndTolerateMissing(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	clientset.PrependReactor("get", "pods", func(action ktesting.Action) (bool, runtime.Object, error) {
		generic := action.(ktesting.GenericAction)
		opts := generic.GetValue().(*corev1.PodLogOptions)
		if !opts.Previous || !opts.Timestamps || opts.TailLines == nil || *opts.TailLines != 100 {
			t.Fatalf("unexpected log options: %#v", opts)
		}
		if opts.Container == "sidecar" {
			return true, nil, errors.New("previous terminated container not found")
		}
		return true, &runtime.Unknown{Raw: []byte("2026-08-17T12:00:00Z log for " + opts.Container)}, nil
	})
	client := &Client{core: clientset}
	pod := diagnosticTestPod(time.Now())
	pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{Name: "sidecar"})
	logs := client.collectPreviousContainerLogs(context.Background(), pod, RestartDiagnosticsOptions{}.withDefaults())
	if len(logs) != 3 || logs[0].Container != "setup" || !logs[0].Init || !logs[0].Available || logs[2].Container != "sidecar" || logs[2].Available {
		t.Fatalf("unexpected logs: %#v", logs)
	}
}

func TestEventFilteringAndProbeClassification(t *testing.T) {
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	pod := diagnosticTestPod(now)
	client := &Client{core: fake.NewSimpleClientset(
		&corev1.Event{ObjectMeta: metav1.ObjectMeta{Namespace: "apps", Name: "probe"}, InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: pod.Name, UID: pod.UID}, Reason: "Unhealthy", Message: "Liveness probe failed", LastTimestamp: metav1.NewTime(now.Add(-time.Second))},
		&corev1.Event{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "pressure"}, InvolvedObject: corev1.ObjectReference{Kind: "Node", Name: "node-a"}, Reason: "MemoryPressure", Message: "node has memory pressure", LastTimestamp: metav1.NewTime(now.Add(-2 * time.Second))},
		&corev1.Event{ObjectMeta: metav1.ObjectMeta{Namespace: "apps", Name: "other"}, InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "other"}, Reason: "Pulled", LastTimestamp: metav1.NewTime(now)},
	)}
	events, storageEvents, err := client.collectRestartDiagnosticEvents(context.Background(), restartIncident{pod: pod, at: now}, nil, 20)
	if err != nil || len(events) != 2 || !hasProbeFailure(events) {
		t.Fatalf("unexpected filtered events: %#v, %v", events, err)
	}
	if len(storageEvents) != 0 {
		t.Fatalf("expected no storage events, got %#v", storageEvents)
	}
	if events[0].Object != "Node/node-a" || events[1].Object != "Pod/api-1" {
		t.Fatalf("events must be chronological: %#v", events)
	}
}

func TestPersistentStorageAndEventsCapture(t *testing.T) {
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	pod := diagnosticTestPod(now)
	pod.Spec.Volumes = []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "data-pvc"}}}}
	volumeMode := corev1.PersistentVolumeFilesystem
	client := &Client{core: fake.NewSimpleClientset(
		&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Namespace: "apps", Name: "data-pvc", UID: typesUID("pvc-uid")},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, VolumeName: "data-pv", VolumeMode: &volumeMode,
				Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}},
			},
			Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound, Capacity: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("2Gi")}},
		},
		&corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: "data-pv", UID: typesUID("pv-uid")}, Spec: corev1.PersistentVolumeSpec{StorageClassName: "fast"}, Status: corev1.PersistentVolumeStatus{Phase: corev1.VolumeBound}},
		&corev1.Event{ObjectMeta: metav1.ObjectMeta{Namespace: "apps", Name: "pvc-event"}, InvolvedObject: corev1.ObjectReference{Kind: "PersistentVolumeClaim", Namespace: "apps", Name: "data-pvc", UID: typesUID("pvc-uid")}, Reason: "ProvisioningSucceeded", Message: "provisioned", LastTimestamp: metav1.NewTime(now)},
	)}
	storage, refs, warnings := client.collectRestartDiagnosticStorage(context.Background(), pod)
	if len(storage) != 1 || storage[0].VolumeName != "data-pv" || storage[0].CapacityBytes != 2*1024*1024*1024 || len(warnings) != 0 {
		t.Fatalf("unexpected persistent storage snapshot: %#v warnings=%#v", storage, warnings)
	}
	_, storageEvents, err := client.collectRestartDiagnosticEvents(context.Background(), restartIncident{pod: pod, at: now}, refs, 20)
	if err != nil || len(storageEvents) != 1 || storageEvents[0].Object != "PersistentVolumeClaim/data-pvc" {
		t.Fatalf("unexpected storage events: %#v, %v", storageEvents, err)
	}
}

func TestResolvePodWorkloadOwnership(t *testing.T) {
	controller := true
	deploymentUID := typesUID("deployment-uid")
	client := &Client{core: fake.NewSimpleClientset(
		&appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{Namespace: "apps", Name: "api-rs", OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "Deployment", Name: "api", UID: deploymentUID, Controller: &controller}}}},
		&batchv1.Job{ObjectMeta: metav1.ObjectMeta{Namespace: "apps", Name: "backup-1", OwnerReferences: []metav1.OwnerReference{{APIVersion: "batch/v1", Kind: "CronJob", Name: "backup", UID: typesUID("cron-uid"), Controller: &controller}}}},
	)}
	pod := diagnosticTestPod(time.Now())
	pod.OwnerReferences = []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "ReplicaSet", Name: "api-rs", UID: typesUID("rs-uid"), Controller: &controller}}
	workload := client.resolvePodWorkload(context.Background(), pod)
	if workload.Kind != "Deployment" || workload.Name != "api" || workload.UID != deploymentUID {
		t.Fatalf("expected Deployment owner, got %#v", workload)
	}
	refs := client.podWorkloadRefs(context.Background(), "apps", []corev1.Pod{*pod})
	if refs[pod.Name].Kind != "Deployment" || refs[pod.Name].Name != "api" {
		t.Fatalf("expected batched pod identity to match Deployment, got %#v", refs)
	}
	pod.OwnerReferences = []metav1.OwnerReference{{APIVersion: "batch/v1", Kind: "Job", Name: "backup-1", UID: typesUID("job-uid"), Controller: &controller}}
	workload = client.resolvePodWorkload(context.Background(), pod)
	if workload.Kind != "CronJob" || workload.Name != "backup" {
		t.Fatalf("expected CronJob owner, got %#v", workload)
	}
}

func diagnosticTestPod(now time.Time) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "apps", Name: "api-1", UID: typesUID("pod-uid")},
		Spec:       corev1.PodSpec{NodeName: "node-a", InitContainers: []corev1.Container{{Name: "setup"}}, Containers: []corev1.Container{{Name: "app"}}},
		Status: corev1.PodStatus{
			ContainerStatuses:     []corev1.ContainerStatus{{Name: "app", RestartCount: 1}},
			InitContainerStatuses: []corev1.ContainerStatus{{Name: "setup"}},
		},
	}
}

func diagnosticTestSnapshot(at time.Time) RestartDiagnosticSnapshot {
	return RestartDiagnosticSnapshot{
		Version: 1, IncidentTime: at, Classification: "oom-killed", Reason: "OOMKilled",
		Workload:  RestartDiagnosticWorkload{APIVersion: "apps/v1", Kind: "Deployment", Namespace: "apps", Name: "api", UID: typesUID("deployment-uid")},
		Pod:       RestartDiagnosticPod{Namespace: "apps", Name: "api-1", UID: typesUID("pod-uid"), Node: "node-a"},
		Container: RestartDiagnosticContainer{Name: "app", RestartCount: 2},
	}
}

func typesUID(value string) types.UID { return types.UID(value) }
