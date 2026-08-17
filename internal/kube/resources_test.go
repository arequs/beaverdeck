package kube

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
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

func TestPodDisplayStatusTerminating(t *testing.T) {
	now := metav1.Now()
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &now}, Status: corev1.PodStatus{Phase: corev1.PodRunning}}
	if got := podDisplayStatus(pod); got != "Terminating" {
		t.Fatalf("expected Terminating, got %q", got)
	}
}
