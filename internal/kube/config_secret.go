package kube

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ConfigSecretRef struct {
	Namespace string
	Name      string
	Key       string
}

func (r ConfigSecretRef) String() string {
	return fmt.Sprintf("%s/%s:%s", r.Namespace, r.Name, r.Key)
}

func (r ConfigSecretRef) valid() error {
	if strings.TrimSpace(r.Namespace) == "" {
		return fmt.Errorf("config secret namespace is required")
	}
	if strings.TrimSpace(r.Name) == "" {
		return fmt.Errorf("config secret name is required")
	}
	if strings.TrimSpace(r.Key) == "" {
		return fmt.Errorf("config secret key is required")
	}
	return nil
}

func (c *Client) GetConfigSecretData(ctx context.Context, ref ConfigSecretRef) ([]byte, bool, error) {
	if err := ref.valid(); err != nil {
		return nil, false, err
	}
	secret, err := c.core.CoreV1().Secrets(ref.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if secret.Data == nil {
		return nil, true, nil
	}
	data := secret.Data[ref.Key]
	if data == nil {
		return nil, true, nil
	}
	out := make([]byte, len(data))
	copy(out, data)
	return out, true, nil
}

func (c *Client) UpsertConfigSecretData(ctx context.Context, ref ConfigSecretRef, data []byte) error {
	if err := ref.valid(); err != nil {
		return err
	}
	secretClient := c.core.CoreV1().Secrets(ref.Namespace)
	secret, err := secretClient.Get(ctx, ref.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ref.Name,
				Namespace: ref.Namespace,
				Labels: map[string]string{
					"app.kubernetes.io/name":      "beaverdeck",
					"app.kubernetes.io/component": "config",
				},
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{ref.Key: data},
		}
		_, err = secretClient.Create(ctx, secret, metav1.CreateOptions{})
		return err
	}
	if err != nil {
		return err
	}
	if secret.Data == nil {
		secret.Data = map[string][]byte{}
	}
	secret.Data[ref.Key] = data
	if secret.Labels == nil {
		secret.Labels = map[string]string{}
	}
	secret.Labels["app.kubernetes.io/name"] = "beaverdeck"
	secret.Labels["app.kubernetes.io/component"] = "config"
	if secret.Type == "" {
		secret.Type = corev1.SecretTypeOpaque
	}
	_, err = secretClient.Update(ctx, secret, metav1.UpdateOptions{})
	return err
}
