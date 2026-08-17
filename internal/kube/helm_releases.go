package kube

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

const (
	helmReleaseLabelSelector   = "owner=helm"
	maxHelmReleaseDecodedBytes = 32 * 1024 * 1024
)

type storedHelmChart struct {
	Metadata struct {
		Name       string `json:"name"`
		Version    string `json:"version"`
		AppVersion string `json:"appVersion"`
	} `json:"metadata"`
	Values       map[string]any    `json:"values"`
	Dependencies []storedHelmChart `json:"dependencies"`
}

type storedHelmRelease struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Version   int    `json:"version"`
	Info      struct {
		FirstDeployed string `json:"first_deployed"`
		LastDeployed  string `json:"last_deployed"`
		Status        string `json:"status"`
		Description   string `json:"description"`
	} `json:"info"`
	Chart    storedHelmChart `json:"chart"`
	Config   map[string]any  `json:"config"`
	Manifest string          `json:"manifest"`
}

func (c *Client) ListHelmReleases(ctx context.Context, namespace string) ([]HelmReleaseInfo, error) {
	revisions, err := c.listHelmReleaseRevisions(ctx, namespace)
	if err != nil {
		return nil, err
	}
	latest := make(map[string]HelmReleaseInfo)
	for _, release := range revisions {
		current, exists := latest[release.Name]
		if !exists || release.Revision > current.Revision {
			latest[release.Name] = release
		}
	}
	out := make([]HelmReleaseInfo, 0, len(latest))
	for _, release := range latest {
		out = append(out, release)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (c *Client) ListHelmReleaseHistory(ctx context.Context, namespace, name string) ([]HelmReleaseInfo, error) {
	revisions, err := c.listHelmReleaseRevisions(ctx, namespace)
	if err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	out := make([]HelmReleaseInfo, 0)
	for _, release := range revisions {
		if release.Name == name {
			out = append(out, release)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Revision > out[j].Revision })
	return out, nil
}

func (c *Client) listHelmReleaseRevisions(ctx context.Context, namespace string) ([]HelmReleaseInfo, error) {
	storedReleases, err := c.listStoredHelmReleases(ctx, namespace)
	if err != nil {
		return nil, err
	}
	out := make([]HelmReleaseInfo, 0, len(storedReleases))
	for _, release := range storedReleases {
		out = append(out, helmReleaseInfo(release))
	}
	return out, nil
}

func (c *Client) listStoredHelmReleases(ctx context.Context, namespace string) ([]storedHelmRelease, error) {
	encoded := make([][]byte, 0)
	secrets, err := c.core.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{LabelSelector: helmReleaseLabelSelector})
	if err != nil {
		return nil, fmt.Errorf("list Helm release Secrets in %s: %w", namespace, err)
	}
	for _, secret := range secrets.Items {
		if releaseData := secret.Data["release"]; len(releaseData) > 0 {
			encoded = append(encoded, releaseData)
		}
	}
	configMaps, err := c.core.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{LabelSelector: helmReleaseLabelSelector})
	if err != nil {
		return nil, fmt.Errorf("list Helm release ConfigMaps in %s: %w", namespace, err)
	}
	for _, configMap := range configMaps.Items {
		if releaseData := strings.TrimSpace(configMap.Data["release"]); releaseData != "" {
			encoded = append(encoded, []byte(releaseData))
		}
	}
	seen := make(map[string]struct{})
	out := make([]storedHelmRelease, 0, len(encoded))
	var firstDecodeErr error
	for _, releaseData := range encoded {
		release, err := decodeStoredHelmRelease(releaseData)
		if err != nil {
			if firstDecodeErr == nil {
				firstDecodeErr = err
			}
			continue
		}
		if release.Namespace == "" {
			release.Namespace = namespace
		}
		key := fmt.Sprintf("%s/%s/%d", release.Namespace, release.Name, release.Version)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, release)
	}
	if len(out) == 0 && firstDecodeErr != nil {
		return nil, fmt.Errorf("decode Helm releases in %s: %w", namespace, firstDecodeErr)
	}
	return out, nil
}

func decodeHelmRelease(encoded []byte) (HelmReleaseInfo, error) {
	stored, err := decodeStoredHelmRelease(encoded)
	if err != nil {
		return HelmReleaseInfo{}, err
	}
	return helmReleaseInfo(stored), nil
}

func decodeStoredHelmRelease(encoded []byte) (storedHelmRelease, error) {
	compressed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(encoded)))
	if err != nil {
		return storedHelmRelease{}, fmt.Errorf("base64: %w", err)
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return storedHelmRelease{}, fmt.Errorf("gzip: %w", err)
	}
	data, err := io.ReadAll(io.LimitReader(reader, maxHelmReleaseDecodedBytes+1))
	closeErr := reader.Close()
	if err != nil {
		return storedHelmRelease{}, fmt.Errorf("read gzip: %w", err)
	}
	if closeErr != nil {
		return storedHelmRelease{}, fmt.Errorf("close gzip: %w", closeErr)
	}
	if len(data) > maxHelmReleaseDecodedBytes {
		return storedHelmRelease{}, fmt.Errorf("decoded release exceeds %d bytes", maxHelmReleaseDecodedBytes)
	}
	var stored storedHelmRelease
	if err := json.Unmarshal(data, &stored); err != nil {
		return storedHelmRelease{}, fmt.Errorf("json: %w", err)
	}
	if strings.TrimSpace(stored.Name) == "" || stored.Version < 1 {
		return storedHelmRelease{}, fmt.Errorf("release name and revision are required")
	}
	return stored, nil
}

func helmReleaseInfo(stored storedHelmRelease) HelmReleaseInfo {
	updated := stored.Info.LastDeployed
	if updated == "" {
		updated = stored.Info.FirstDeployed
	}
	return HelmReleaseInfo{
		Namespace: stored.Namespace, Name: stored.Name, Revision: stored.Version,
		Status: stored.Info.Status, Chart: stored.Chart.Metadata.Name, ChartVersion: stored.Chart.Metadata.Version,
		AppVersion: stored.Chart.Metadata.AppVersion, Updated: updated, Description: stored.Info.Description,
	}
}

func (c *Client) GetHelmReleaseRevisionYAML(ctx context.Context, namespace, name string, revision int, detail string) (string, error) {
	releases, err := c.listStoredHelmReleases(ctx, namespace)
	if err != nil {
		return "", err
	}
	for _, release := range releases {
		if release.Name != name || release.Version != revision {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(detail)) {
		case "values":
			return marshalHelmValues(computedHelmValues(release.Chart, release.Config))
		case "user-values":
			return marshalHelmValues(release.Config)
		case "resources":
			return release.Manifest, nil
		default:
			return "", fmt.Errorf("unsupported Helm revision detail %q", detail)
		}
	}
	return "", fmt.Errorf("Helm release %s revision %d not found in namespace %s", name, revision, namespace)
}

func marshalHelmValues(values map[string]any) (string, error) {
	if values == nil {
		values = map[string]any{}
	}
	data, err := yaml.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("marshal Helm values: %w", err)
	}
	return string(data), nil
}

func computedHelmValues(chart storedHelmChart, userValues map[string]any) map[string]any {
	values := cloneHelmMap(chart.Values)
	for _, dependency := range chart.Dependencies {
		name := strings.TrimSpace(dependency.Metadata.Name)
		if name == "" {
			continue
		}
		childOverrides, _ := values[name].(map[string]any)
		values[name] = mergeHelmMaps(computedHelmValues(dependency, childOverrides), childOverrides)
	}
	return mergeHelmMaps(values, userValues)
}

func mergeHelmMaps(base, override map[string]any) map[string]any {
	out := cloneHelmMap(base)
	for key, value := range override {
		baseMap, baseOK := out[key].(map[string]any)
		overrideMap, overrideOK := value.(map[string]any)
		if baseOK && overrideOK {
			out[key] = mergeHelmMaps(baseMap, overrideMap)
			continue
		}
		out[key] = cloneHelmValue(value)
	}
	return out
}

func cloneHelmMap(input map[string]any) map[string]any {
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = cloneHelmValue(value)
	}
	return out
}

func cloneHelmValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneHelmMap(typed)
	case []any:
		out := make([]any, len(typed))
		for index, item := range typed {
			out[index] = cloneHelmValue(item)
		}
		return out
	default:
		return value
	}
}
