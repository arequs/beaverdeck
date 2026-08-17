package kube

import (
	"context"
	"strings"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestListArgoCDApplicationsHistoryAndDetails(t *testing.T) {
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(),
		map[schema.GroupVersionResource]string{argoCDApplicationGVR: "ApplicationList"},
		argoCDApplicationObject("apps", "payments"),
		argoCDApplicationObject("restricted", "hidden"),
	)
	client := &Client{dyn: dynamicClient}

	applications, err := client.ListArgoCDApplications(context.Background(), "apps")
	if err != nil {
		t.Fatal(err)
	}
	if len(applications) != 1 {
		t.Fatalf("got %d applications, want 1: %#v", len(applications), applications)
	}
	application := applications[0]
	if application.Name != "payments" || application.Project != "platform" || application.SyncStatus != "Synced" || application.HealthStatus != "Healthy" {
		t.Fatalf("unexpected application: %#v", application)
	}
	if application.Revision != "abc123" || application.Source != "deploy/payments" || application.Destination != "in-cluster / workloads" {
		t.Fatalf("unexpected application source data: %#v", application)
	}

	history, err := client.ListArgoCDApplicationHistory(context.Background(), "apps", "payments")
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 || history[0].ID != 1 || history[1].ID != 0 {
		t.Fatalf("unexpected history order: %#v", history)
	}
	if !history[0].Current || history[1].Current || history[0].InitiatedBy != "automated" {
		t.Fatalf("unexpected history state: %#v", history)
	}

	source, err := client.GetArgoCDApplicationRevisionYAML(context.Background(), "apps", "payments", 1, "source")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(source, "historyID: 1") || !strings.Contains(source, "repoURL: https://github.com/example/platform.git") || !strings.Contains(source, "revision: abc123") {
		t.Fatalf("unexpected source YAML:\n%s", source)
	}

	resources, err := client.GetArgoCDApplicationRevisionYAML(context.Background(), "apps", "payments", 1, "resources")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resources, "kind: Deployment") || !strings.Contains(resources, "name: payments") || !strings.Contains(resources, "status: Synced") {
		t.Fatalf("unexpected resources YAML:\n%s", resources)
	}

	if _, err := client.GetArgoCDApplicationRevisionYAML(context.Background(), "apps", "payments", 0, "resources"); err == nil || !strings.Contains(err.Error(), "current") {
		t.Fatalf("expected historical resources to be rejected, got %v", err)
	}
}

func TestListArgoCDApplicationsWithoutCRDReturnsEmpty(t *testing.T) {
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(),
		map[schema.GroupVersionResource]string{argoCDApplicationGVR: "ApplicationList"},
	)
	dynamicClient.PrependReactor("list", "applications", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewNotFound(argoCDApplicationGVR.GroupResource(), "")
	})
	client := &Client{dyn: dynamicClient}

	applications, err := client.ListArgoCDApplications(context.Background(), "apps")
	if err != nil {
		t.Fatal(err)
	}
	if len(applications) != 0 {
		t.Fatalf("got %#v, want an empty list", applications)
	}
}

func argoCDApplicationObject(namespace, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Application",
		"metadata": map[string]any{
			"name":              name,
			"namespace":         namespace,
			"creationTimestamp": "2026-08-14T10:00:00Z",
		},
		"spec": map[string]any{
			"project": "platform",
			"source": map[string]any{
				"repoURL":        "https://github.com/example/platform.git",
				"path":           "deploy/payments",
				"targetRevision": "main",
			},
			"destination": map[string]any{
				"name":      "in-cluster",
				"namespace": "workloads",
			},
		},
		"status": map[string]any{
			"sync":         map[string]any{"status": "Synced", "revision": "abc123"},
			"health":       map[string]any{"status": "Healthy"},
			"reconciledAt": "2026-08-16T10:00:00Z",
			"history": []any{
				map[string]any{
					"id": int64(0), "revision": "old456", "deployedAt": "2026-08-15T10:00:00Z",
					"source":      map[string]any{"repoURL": "https://github.com/example/platform.git", "path": "deploy/payments", "targetRevision": "main"},
					"initiatedBy": map[string]any{"username": "operator@example.com"},
				},
				map[string]any{
					"id": int64(1), "revision": "abc123", "deployStartedAt": "2026-08-16T09:59:00Z", "deployedAt": "2026-08-16T10:00:00Z",
					"source":      map[string]any{"repoURL": "https://github.com/example/platform.git", "path": "deploy/payments", "targetRevision": "main"},
					"initiatedBy": map[string]any{"automated": true},
				},
			},
			"resources": []any{
				map[string]any{
					"group": "apps", "version": "v1", "kind": "Deployment", "namespace": "workloads", "name": name,
					"status": "Synced", "health": map[string]any{"status": "Healthy"},
				},
			},
		},
	}}
}
