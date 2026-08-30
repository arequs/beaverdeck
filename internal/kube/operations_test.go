package kube

import (
	"context"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetSecretManifestYAMLIncludesBase64Data(t *testing.T) {
	client := &Client{core: fake.NewSimpleClientset(testSecret())}

	text, err := client.GetManifestYAML(context.Background(), "apps", "secret", "app-secret")
	if err != nil {
		t.Fatalf("GetManifestYAML returned error: %v", err)
	}

	if !strings.Contains(text, "username: YWRtaW4=") || !strings.Contains(text, "password: cEBzcw==") {
		t.Fatalf("expected secret manifest to include Kubernetes base64 data, got:\n%s", text)
	}
}

func TestGetSecretDecodedDataReturnsDecodedValues(t *testing.T) {
	client := &Client{core: fake.NewSimpleClientset(testSecret())}

	decoded, err := client.GetSecretDecodedData(context.Background(), "apps", "app-secret")
	if err != nil {
		t.Fatalf("GetSecretDecodedData returned error: %v", err)
	}

	if decoded["username"] != "admin" || decoded["password"] != "p@ss" {
		t.Fatalf("expected decoded secret values, got: %#v", decoded)
	}
}

func TestWorkloadPodsUsesSelectorAndSortsMatches(t *testing.T) {
	client := &Client{core: fake.NewSimpleClientset(
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Namespace: "apps", Name: "demo"},
			Spec:       appsv1.DeploymentSpec{Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "demo"}}},
		},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "apps", Name: "demo-b", Labels: map[string]string{"app": "demo"}}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "apps", Name: "demo-a", Labels: map[string]string{"app": "demo"}}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "apps", Name: "other", Labels: map[string]string{"app": "other"}}},
	)}

	pods, err := client.workloadPods(context.Background(), "apps", "Deployment", "demo")
	if err != nil {
		t.Fatal(err)
	}
	if len(pods) != 2 || pods[0].Name != "demo-a" || pods[1].Name != "demo-b" {
		t.Fatalf("workload pods = %#v, want demo-a and demo-b", pods)
	}
}

func testSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "apps",
			Name:      "app-secret",
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"username": []byte("admin"),
			"password": []byte("p@ss"),
		},
	}
}
