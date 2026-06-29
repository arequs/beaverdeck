package kube

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SuppressedInsightsRef struct {
	Namespace string
	Name      string
	Key       string
}

func (r SuppressedInsightsRef) String() string {
	return fmt.Sprintf("%s/%s:%s", r.Namespace, r.Name, r.Key)
}

func (r SuppressedInsightsRef) valid() error {
	if strings.TrimSpace(r.Namespace) == "" {
		return fmt.Errorf("suppressed insights configmap namespace is required")
	}
	if strings.TrimSpace(r.Name) == "" {
		return fmt.Errorf("suppressed insights configmap name is required")
	}
	if strings.TrimSpace(r.Key) == "" {
		return fmt.Errorf("suppressed insights configmap key is required")
	}
	return nil
}

func (c *Client) ListSuppressedInsights(ctx context.Context, ref SuppressedInsightsRef) ([]string, error) {
	data, found, err := c.getSuppressedInsightsData(ctx, ref)
	if err != nil {
		return nil, err
	}
	if !found || strings.TrimSpace(string(data)) == "" {
		return []string{}, nil
	}
	var keys []string
	if err := json.Unmarshal(data, &keys); err != nil {
		return nil, fmt.Errorf("decode suppressed insights configmap %s: %w", ref.String(), err)
	}
	return normalizeSuppressedInsightKeys(keys)
}

func (c *Client) SetInsightSuppressed(ctx context.Context, ref SuppressedInsightsRef, key string, suppressed bool) error {
	key, err := cleanSuppressedInsightKey(key)
	if err != nil {
		return err
	}
	current, err := c.ListSuppressedInsights(ctx, ref)
	if err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(current)+1)
	next := make([]string, 0, len(current)+1)
	for _, item := range current {
		if item == key {
			continue
		}
		seen[item] = struct{}{}
		next = append(next, item)
	}
	if suppressed {
		if _, ok := seen[key]; !ok {
			next = append(next, key)
		}
	}
	next, err = normalizeSuppressedInsightKeys(next)
	if err != nil {
		return err
	}
	data, err := json.Marshal(next)
	if err != nil {
		return err
	}
	return c.upsertSuppressedInsightsData(ctx, ref, data)
}

func (c *Client) getSuppressedInsightsData(ctx context.Context, ref SuppressedInsightsRef) ([]byte, bool, error) {
	if err := ref.valid(); err != nil {
		return nil, false, err
	}
	cm, err := c.core.CoreV1().ConfigMaps(ref.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if cm.Data == nil {
		return nil, true, nil
	}
	data, ok := cm.Data[ref.Key]
	if !ok {
		return nil, true, nil
	}
	return []byte(data), true, nil
}

func (c *Client) upsertSuppressedInsightsData(ctx context.Context, ref SuppressedInsightsRef, data []byte) error {
	if err := ref.valid(); err != nil {
		return err
	}
	configMapClient := c.core.CoreV1().ConfigMaps(ref.Namespace)
	cm, err := configMapClient.Get(ctx, ref.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		cm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ref.Name,
				Namespace: ref.Namespace,
				Labels: map[string]string{
					"app.kubernetes.io/name":      "beaverdeck",
					"app.kubernetes.io/component": "insights",
				},
			},
			Data: map[string]string{ref.Key: string(data)},
		}
		_, err = configMapClient.Create(ctx, cm, metav1.CreateOptions{})
		return err
	}
	if err != nil {
		return err
	}
	if cm.Data == nil {
		cm.Data = map[string]string{}
	}
	cm.Data[ref.Key] = string(data)
	if cm.Labels == nil {
		cm.Labels = map[string]string{}
	}
	cm.Labels["app.kubernetes.io/name"] = "beaverdeck"
	cm.Labels["app.kubernetes.io/component"] = "insights"
	_, err = configMapClient.Update(ctx, cm, metav1.UpdateOptions{})
	return err
}

func normalizeSuppressedInsightKeys(input []string) ([]string, error) {
	seen := make(map[string]struct{}, len(input))
	out := make([]string, 0, len(input))
	for _, item := range input {
		key, err := cleanSuppressedInsightKey(item)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	sort.Strings(out)
	return out, nil
}

func cleanSuppressedInsightKey(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("suppressed insight key is required")
	}
	if len(value) > 512 {
		return "", fmt.Errorf("suppressed insight key is too long")
	}
	for _, r := range value {
		if r == '\x00' || r == '\r' || r == '\n' || (unicode.IsControl(r) && r != '\t') {
			return "", fmt.Errorf("suppressed insight key contains control characters")
		}
	}
	return value, nil
}
