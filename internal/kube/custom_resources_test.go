package kube

import (
	"context"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
)

func TestResolveAndListNamespacedCustomResources(t *testing.T) {
	crdGVR := schema.GroupVersionResource{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"}
	widgetGVR := schema.GroupVersionResource{Group: "example.io", Version: "v1", Resource: "widgets"}
	objects := []runtime.Object{
		customResourceDefinitionObject(),
		customResourceObject("apps", "visible-widget"),
		customResourceObject("restricted", "hidden-widget"),
	}
	dynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
		crdGVR:    "CustomResourceDefinitionList",
		widgetGVR: "WidgetList",
	}, objects...)
	client := &Client{dyn: dynamicClient}

	definition, err := client.ResolveCustomResourceDefinition(context.Background(), "widgets.example.io")
	if err != nil {
		t.Fatal(err)
	}
	if !definition.Namespaced || definition.Group != "example.io" || definition.Version != "v1" || definition.Resource != "widgets" || definition.Kind != "Widget" {
		t.Fatalf("unexpected definition: %#v", definition)
	}

	items, err := client.ListCustomResources(context.Background(), definition, []string{"apps"})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Namespace != "apps" || items[0].Name != "visible-widget" {
		t.Fatalf("expected only the allowed namespace resource, got %#v", items)
	}
}

func customResourceDefinitionObject() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apiextensions.k8s.io/v1",
		"kind":       "CustomResourceDefinition",
		"metadata": map[string]any{
			"name": "widgets.example.io",
		},
		"spec": map[string]any{
			"group": "example.io",
			"scope": "Namespaced",
			"names": map[string]any{"kind": "Widget", "plural": "widgets"},
			"versions": []any{
				map[string]any{"name": "v1beta1", "served": true, "storage": false},
				map[string]any{"name": "v1", "served": true, "storage": true},
			},
		},
	}}
}

func customResourceObject(namespace, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "example.io/v1",
		"kind":       "Widget",
		"metadata": map[string]any{
			"namespace":         namespace,
			"name":              name,
			"creationTimestamp": time.Now().UTC().Format(time.RFC3339),
		},
		"spec": map[string]any{"enabled": true},
	}}
}
