package kube

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"strconv"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestListHelmReleasesAndHistory(t *testing.T) {
	client := &Client{core: fake.NewSimpleClientset(
		helmReleaseSecret(t, "demo", "sample", 1, "superseded", "1.0.0"),
		helmReleaseSecret(t, "demo", "sample", 2, "deployed", "1.1.0"),
		helmReleaseConfigMap(t, "demo", "other", 3, "failed", "3.0.0"),
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "sh.helm.release.v1.corrupt.v1", Labels: map[string]string{"owner": "helm"}},
			Data:       map[string][]byte{"release": []byte("not-a-release")},
		},
		helmReleaseSecret(t, "restricted", "hidden", 1, "deployed", "9.0.0"),
	)}

	releases, err := client.ListHelmReleases(context.Background(), "demo")
	if err != nil {
		t.Fatal(err)
	}
	if len(releases) != 2 {
		t.Fatalf("got %d releases, want 2: %#v", len(releases), releases)
	}
	if releases[0].Name != "other" || releases[0].Status != "failed" {
		t.Fatalf("unexpected first release: %#v", releases[0])
	}
	if releases[1].Name != "sample" || releases[1].Revision != 2 || releases[1].Status != "deployed" || releases[1].ChartVersion != "1.1.0" {
		t.Fatalf("unexpected current sample release: %#v", releases[1])
	}

	history, err := client.ListHelmReleaseHistory(context.Background(), "demo", "sample")
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 || history[0].Revision != 2 || history[1].Revision != 1 {
		t.Fatalf("unexpected history: %#v", history)
	}
	values, err := client.GetHelmReleaseRevisionYAML(context.Background(), "demo", "sample", 2, "values")
	if err != nil {
		t.Fatal(err)
	}
	if values != "image: nginx:1.27\nreplicas: 2\n" {
		t.Fatalf("unexpected computed values:\n%s", values)
	}
	userValues, err := client.GetHelmReleaseRevisionYAML(context.Background(), "demo", "sample", 2, "user-values")
	if err != nil {
		t.Fatal(err)
	}
	if userValues != "replicas: 2\n" {
		t.Fatalf("unexpected user values:\n%s", userValues)
	}
	resources, err := client.GetHelmReleaseRevisionYAML(context.Background(), "demo", "sample", 2, "resources")
	if err != nil {
		t.Fatal(err)
	}
	if resources != "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: sample\n" {
		t.Fatalf("unexpected resources:\n%s", resources)
	}
}

func TestListHelmReleasesReportsWhenEveryStoredReleaseIsCorrupt(t *testing.T) {
	client := &Client{core: fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "sh.helm.release.v1.corrupt.v1", Labels: map[string]string{"owner": "helm"}},
		Data:       map[string][]byte{"release": []byte("not-a-release")},
	})}

	if _, err := client.ListHelmReleases(context.Background(), "demo"); err == nil {
		t.Fatal("expected an error when every stored Helm release is corrupt")
	}
}

func helmReleaseSecret(t *testing.T, namespace, name string, revision int, status, chartVersion string) *corev1.Secret {
	t.Helper()
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      helmStorageName(name, revision),
			Labels:    map[string]string{"owner": "helm", "name": name, "status": status},
		},
		Type: corev1.SecretType("helm.sh/release.v1"),
		Data: map[string][]byte{"release": encodedHelmRelease(t, namespace, name, revision, status, chartVersion)},
	}
}

func helmReleaseConfigMap(t *testing.T, namespace, name string, revision int, status, chartVersion string) *corev1.ConfigMap {
	t.Helper()
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      helmStorageName(name, revision),
			Labels:    map[string]string{"owner": "helm", "name": name, "status": status},
		},
		Data: map[string]string{"release": string(encodedHelmRelease(t, namespace, name, revision, status, chartVersion))},
	}
}

func helmStorageName(name string, revision int) string {
	return "sh.helm.release.v1." + name + ".v" + strconv.Itoa(revision)
}

func encodedHelmRelease(t *testing.T, namespace, name string, revision int, status, chartVersion string) []byte {
	t.Helper()
	release := map[string]any{
		"name": name, "namespace": namespace, "version": revision,
		"config":   map[string]any{"replicas": 2},
		"manifest": "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: sample\n",
		"info": map[string]any{
			"first_deployed": "2026-08-15T10:00:00Z", "last_deployed": "2026-08-16T10:00:00Z",
			"status": status, "description": "test release",
		},
		"chart": map[string]any{
			"metadata": map[string]any{"name": name + "-chart", "version": chartVersion, "appVersion": "2.0.0"},
			"values":   map[string]any{"image": "nginx:1.27", "replicas": 1},
		},
	}
	data, err := json.Marshal(release)
	if err != nil {
		t.Fatal(err)
	}
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	encoded := make([]byte, base64.StdEncoding.EncodedLen(compressed.Len()))
	base64.StdEncoding.Encode(encoded, compressed.Bytes())
	return encoded
}
