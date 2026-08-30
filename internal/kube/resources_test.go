package kube

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestPodDisplayStatusUsesContainerFailureReason(t *testing.T) {
	pod := &corev1.Pod{Status: corev1.PodStatus{
		Phase: corev1.PodRunning,
		ContainerStatuses: []corev1.ContainerStatus{{
			Name: "app", Ready: false, RestartCount: 35,
			State:                corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
			LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "OOMKilled", ExitCode: 137}},
		}},
	}}
	if got := podDisplayStatus(pod); got != "OOMKilled" {
		t.Fatalf("expected OOMKilled instead of phase Running, got %q", got)
	}

	pod.Status.ContainerStatuses[0].LastTerminationState.Terminated = nil
	if got := podDisplayStatus(pod); got != "CrashLoopBackOff" {
		t.Fatalf("expected current waiting reason, got %q", got)
	}

	pod.Status.Reason = "Evicted"
	if got := podDisplayStatus(pod); got != "Evicted" {
		t.Fatalf("expected pod reason, got %q", got)
	}
}

func TestListEventsPreservesInvolvedObjectUID(t *testing.T) {
	client := &Client{core: fake.NewSimpleClientset(&corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Namespace: "apps", Name: "oom"},
		InvolvedObject: corev1.ObjectReference{Kind: "Pod", Namespace: "apps", Name: "oom-test", UID: types.UID("pod-uid")},
		Reason:         "BackOff",
		LastTimestamp:  metav1.Now(),
	})}
	events, err := client.ListEvents(context.Background(), "apps", 20)
	if err != nil || len(events) != 1 || events[0].ObjectUID != types.UID("pod-uid") {
		t.Fatalf("expected event object UID, got %#v, %v", events, err)
	}
}

func TestListEventsByTypeUsesServerSideFieldSelector(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	client := &Client{core: clientset}
	if _, err := client.ListEventsByType(context.Background(), "apps", 30, "Warning"); err != nil {
		t.Fatal(err)
	}
	if len(clientset.Actions()) != 1 {
		t.Fatalf("expected one Kubernetes request, got %#v", clientset.Actions())
	}
	listAction, ok := clientset.Actions()[0].(k8stesting.ListAction)
	if !ok {
		t.Fatalf("expected list action, got %#v", clientset.Actions()[0])
	}
	if got := listAction.GetListRestrictions().Fields.String(); got != "type=Warning" {
		t.Fatalf("event field selector = %q, want type=Warning", got)
	}
}

func TestListEventsSortsByEffectiveKubernetesTimestampBeforeLimiting(t *testing.T) {
	older := time.Now().UTC().Add(-time.Minute)
	newer := older.Add(30 * time.Second)
	client := &Client{core: fake.NewSimpleClientset(
		&corev1.Event{
			ObjectMeta:     metav1.ObjectMeta{Namespace: "apps", Name: "legacy-event"},
			InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "old"},
			LastTimestamp:  metav1.NewTime(older),
		},
		&corev1.Event{
			ObjectMeta:     metav1.ObjectMeta{Namespace: "apps", Name: "events-v1-event"},
			InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "new"},
			EventTime:      metav1.NewMicroTime(newer),
		},
	)}

	events, err := client.ListEvents(context.Background(), "apps", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Object != "Pod/new" {
		t.Fatalf("events = %#v, want the newest EventTime-based event", events)
	}
	if events[0].LastSeen != newer.Format(time.RFC3339) {
		t.Fatalf("last_seen = %q, want %q", events[0].LastSeen, newer.Format(time.RFC3339))
	}
}

func TestPVCAndPVListsCanSkipNodeVolumeMetrics(t *testing.T) {
	clientset := fake.NewSimpleClientset(
		&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "data", Namespace: "apps"}},
		&corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: "pv-data"}},
	)
	client := &Client{core: clientset}
	if _, err := client.ListPVCs(context.Background(), []string{"apps"}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ListPVs(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if got := countListActions(clientset.Actions(), "nodes"); got != 0 {
		t.Fatalf("lightweight PVC/PV inventory listed nodes %d times; actions=%#v", got, clientset.Actions())
	}
	if got := countListActions(clientset.Actions(), "persistentvolumeclaims"); got != 1 {
		t.Fatalf("PVC inventory list count = %d, want 1", got)
	}
	if got := countListActions(clientset.Actions(), "persistentvolumes"); got != 1 {
		t.Fatalf("PV inventory list count = %d, want 1", got)
	}
}

func TestPodDisplayStatusTerminating(t *testing.T) {
	now := metav1.Now()
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &now}, Status: corev1.PodStatus{Phase: corev1.PodRunning}}
	if got := podDisplayStatus(pod); got != "Terminating" {
		t.Fatalf("expected Terminating, got %q", got)
	}
}
