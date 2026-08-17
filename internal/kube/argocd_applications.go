package kube

import (
	"context"
	"fmt"
	"sort"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
)

var argoCDApplicationGVR = schema.GroupVersionResource{
	Group: "argoproj.io", Version: "v1alpha1", Resource: "applications",
}

func (c *Client) ListArgoCDApplications(ctx context.Context, namespace string) ([]ArgoCDApplicationInfo, error) {
	items, err := c.dyn.Resource(argoCDApplicationGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if apierrors.IsNotFound(err) {
		return []ArgoCDApplicationInfo{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list Argo CD Applications in %s: %w", namespace, err)
	}

	out := make([]ArgoCDApplicationInfo, 0, len(items.Items))
	for index := range items.Items {
		out = append(out, argoCDApplicationInfo(&items.Items[index]))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (c *Client) ListArgoCDApplicationHistory(ctx context.Context, namespace, name string) ([]ArgoCDApplicationRevisionInfo, error) {
	application, err := c.getArgoCDApplication(ctx, namespace, name)
	if err != nil {
		return nil, err
	}

	history, _, _ := unstructured.NestedSlice(application.Object, "status", "history")
	currentRevisions := argoCDCurrentRevisions(application)
	out := make([]ArgoCDApplicationRevisionInfo, 0, len(history))
	for _, raw := range history {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		id, ok := nestedInt64(entry, "id")
		if !ok {
			continue
		}
		revisions := argoCDHistoryRevisions(entry)
		out = append(out, ArgoCDApplicationRevisionInfo{
			Namespace:     application.GetNamespace(),
			Name:          application.GetName(),
			ID:            id,
			Revision:      strings.Join(revisions, ", "),
			Source:        argoCDHistorySourceSummary(entry),
			DeployStarted: nestedString(entry, "deployStartedAt"),
			Deployed:      nestedString(entry, "deployedAt"),
			InitiatedBy:   argoCDInitiatedBy(entry),
			Current:       equalStringSlices(revisions, currentRevisions),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out, nil
}

func (c *Client) GetArgoCDApplicationRevisionYAML(ctx context.Context, namespace, name string, revision int64, detail string) (string, error) {
	application, err := c.getArgoCDApplication(ctx, namespace, name)
	if err != nil {
		return "", err
	}
	history, _, _ := unstructured.NestedSlice(application.Object, "status", "history")
	for _, raw := range history {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		id, ok := nestedInt64(entry, "id")
		if !ok || id != revision {
			continue
		}

		var document map[string]any
		switch strings.ToLower(strings.TrimSpace(detail)) {
		case "source":
			document = argoCDRevisionSourceDocument(application, entry, id)
		case "resources":
			revisions := argoCDHistoryRevisions(entry)
			if !equalStringSlices(revisions, argoCDCurrentRevisions(application)) {
				return "", fmt.Errorf("created resources are available only for the current Argo CD Application revision")
			}
			resources, _, _ := unstructured.NestedSlice(application.Object, "status", "resources")
			document = map[string]any{
				"application": application.GetName(),
				"namespace":   application.GetNamespace(),
				"historyID":   id,
				"revision":    revisionValue(argoCDHistoryRevisions(entry)),
				"resources":   resources,
			}
		default:
			return "", fmt.Errorf("unsupported Argo CD Application revision detail %q", detail)
		}
		data, err := yaml.Marshal(document)
		if err != nil {
			return "", fmt.Errorf("marshal Argo CD Application revision detail: %w", err)
		}
		return string(data), nil
	}
	return "", fmt.Errorf("Argo CD Application %s history ID %d not found in namespace %s", name, revision, namespace)
}

func (c *Client) getArgoCDApplication(ctx context.Context, namespace, name string) (*unstructured.Unstructured, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("Argo CD Application name is required")
	}
	application, err := c.dyn.Resource(argoCDApplicationGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get Argo CD Application %s in %s: %w", name, namespace, err)
	}
	return application, nil
}

func argoCDApplicationInfo(application *unstructured.Unstructured) ArgoCDApplicationInfo {
	revisions := argoCDCurrentRevisions(application)
	updated := nestedString(application.Object, "status", "reconciledAt")
	if updated == "" {
		updated = nestedString(application.Object, "status", "operationState", "finishedAt")
	}
	created := application.GetCreationTimestamp()
	if updated == "" && !created.IsZero() {
		updated = created.UTC().Format("2006-01-02T15:04:05Z")
	}
	return ArgoCDApplicationInfo{
		Namespace:    application.GetNamespace(),
		Name:         application.GetName(),
		Project:      nestedString(application.Object, "spec", "project"),
		SyncStatus:   nestedString(application.Object, "status", "sync", "status"),
		HealthStatus: nestedString(application.Object, "status", "health", "status"),
		Revision:     strings.Join(revisions, ", "),
		Source:       argoCDApplicationSourceSummary(application),
		Destination:  argoCDDestinationSummary(application),
		Updated:      updated,
	}
}

func argoCDCurrentRevisions(application *unstructured.Unstructured) []string {
	if revisions, found, _ := unstructured.NestedStringSlice(application.Object, "status", "sync", "revisions"); found {
		return cleanStrings(revisions)
	}
	if revision := nestedString(application.Object, "status", "sync", "revision"); revision != "" {
		return []string{revision}
	}
	return nil
}

func argoCDHistoryRevisions(entry map[string]any) []string {
	if revisions, found, _ := unstructured.NestedStringSlice(entry, "revisions"); found {
		return cleanStrings(revisions)
	}
	if revision := nestedString(entry, "revision"); revision != "" {
		return []string{revision}
	}
	return nil
}

func argoCDApplicationSourceSummary(application *unstructured.Unstructured) string {
	if sources, found, _ := unstructured.NestedSlice(application.Object, "spec", "sources"); found && len(sources) > 0 {
		return argoCDSourcesSummary(sources)
	}
	if source, found, _ := unstructured.NestedMap(application.Object, "spec", "source"); found {
		return argoCDSourceSummary(source)
	}
	return ""
}

func argoCDHistorySourceSummary(entry map[string]any) string {
	if sources, found, _ := unstructured.NestedSlice(entry, "sources"); found && len(sources) > 0 {
		return argoCDSourcesSummary(sources)
	}
	if source, found, _ := unstructured.NestedMap(entry, "source"); found {
		return argoCDSourceSummary(source)
	}
	return ""
}

func argoCDSourcesSummary(sources []any) string {
	parts := make([]string, 0, len(sources))
	for _, raw := range sources {
		if source, ok := raw.(map[string]any); ok {
			if summary := argoCDSourceSummary(source); summary != "" {
				parts = append(parts, summary)
			}
		}
	}
	return strings.Join(parts, ", ")
}

func argoCDSourceSummary(source map[string]any) string {
	location := nestedString(source, "chart")
	if location == "" {
		location = nestedString(source, "path")
	}
	if location == "" {
		location = nestedString(source, "repoURL")
	}
	return location
}

func argoCDDestinationSummary(application *unstructured.Unstructured) string {
	cluster := nestedString(application.Object, "spec", "destination", "name")
	if cluster == "" {
		cluster = nestedString(application.Object, "spec", "destination", "server")
	}
	namespace := nestedString(application.Object, "spec", "destination", "namespace")
	if cluster == "" {
		return namespace
	}
	if namespace == "" {
		return cluster
	}
	return cluster + " / " + namespace
}

func argoCDInitiatedBy(entry map[string]any) string {
	if username := nestedString(entry, "initiatedBy", "username"); username != "" {
		return username
	}
	if automated, found, _ := unstructured.NestedBool(entry, "initiatedBy", "automated"); found && automated {
		return "automated"
	}
	return ""
}

func argoCDRevisionSourceDocument(application *unstructured.Unstructured, entry map[string]any, id int64) map[string]any {
	document := map[string]any{
		"application": application.GetName(),
		"namespace":   application.GetNamespace(),
		"historyID":   id,
		"revision":    revisionValue(argoCDHistoryRevisions(entry)),
	}
	if sources, found, _ := unstructured.NestedSlice(entry, "sources"); found && len(sources) > 0 {
		document["sources"] = sources
	} else if source, found, _ := unstructured.NestedMap(entry, "source"); found {
		document["source"] = source
	}
	return document
}

func revisionValue(revisions []string) any {
	if len(revisions) == 1 {
		return revisions[0]
	}
	return revisions
}

func nestedString(object map[string]any, fields ...string) string {
	value, _, _ := unstructured.NestedString(object, fields...)
	return strings.TrimSpace(value)
}

func nestedInt64(object map[string]any, fields ...string) (int64, bool) {
	value, found, _ := unstructured.NestedInt64(object, fields...)
	return value, found
}

func cleanStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func equalStringSlices(left, right []string) bool {
	if len(left) == 0 || len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
