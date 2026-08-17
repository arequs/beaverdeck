package api

import (
	"context"
	"embed"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"beaverdeck/internal/auth"
	"beaverdeck/internal/config"
	"beaverdeck/internal/users"
)

func TestManifestPermissionAction(t *testing.T) {
	tests := []struct {
		resource string
		want     string
	}{
		{resource: "pods", want: "view"},
		{resource: "workloads", want: "view"},
		{resource: "nodes", want: "view"},
		{resource: "services", want: "view"},
		{resource: "serviceaccounts", want: "view"},
		{resource: "ingresses", want: "view"},
		{resource: "rbacroles", want: "view"},
		{resource: "clusterroles", want: "view"},
		{resource: "configmaps", want: "view"},
		{resource: "crds", want: "view"},
		{resource: "secrets", want: "edit"},
		{resource: "pvcs", want: "view"},
		{resource: "pvs", want: "view"},
		{resource: "storageclasses", want: "view"},
	}

	for _, tt := range tests {
		t.Run(tt.resource, func(t *testing.T) {
			if got := manifestPermissionAction(tt.resource); got != tt.want {
				t.Fatalf("manifest permission for %s = %q, want %q", tt.resource, got, tt.want)
			}
		})
	}
}

func TestRequestedLogTailBounds(t *testing.T) {
	tests := []struct {
		query string
		want  int64
	}{
		{query: "", want: defaultLogTailLines},
		{query: "?tail=0", want: defaultLogTailLines},
		{query: "?tail=250", want: 250},
		{query: "?tail=999999999", want: maxLogTailLines},
	}
	for _, tt := range tests {
		request := httptest.NewRequest(http.MethodGet, "/api/podlogs"+tt.query, nil)
		if got := requestedLogTail(request); got != tt.want {
			t.Fatalf("requestedLogTail(%q) = %d, want %d", tt.query, got, tt.want)
		}
	}
}

func TestAPIRequestBodyLimit(t *testing.T) {
	store, err := users.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.ResetToEmptyConfig(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteBootstrap(context.Background(), "admin", "admin-password"); err != nil {
		t.Fatal(err)
	}
	server := New(config.Config{}, nil, store, embed.FS{})
	body := `{"username":"` + strings.Repeat("x", maxAPIRequestBodyBytes) + `","password":"secret"}`
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("returned status %d, want %d: %s", response.Code, http.StatusBadRequest, response.Body.String())
	}
	if !strings.Contains(strings.ToLower(response.Body.String()), "too large") {
		t.Fatalf("expected body size error, got: %s", response.Body.String())
	}
}

func TestNamespacedListConcurrencyIsBounded(t *testing.T) {
	ctx := context.Background()
	store, err := users.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.ResetToEmptyConfig(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteBootstrap(ctx, "admin", "admin-password"); err != nil {
		t.Fatal(err)
	}
	token, err := store.CreateSession(ctx, "admin", "local")
	if err != nil {
		t.Fatal(err)
	}

	server := New(config.Config{AllowAllNamespaces: true}, nil, store, embed.FS{})
	started := make(chan struct{}, maxNamespaceListConcurrency*2)
	release := make(chan struct{})
	handler := auth.Middleware(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeNamespacedList(server, w, r, "pods", func(_ context.Context, namespace string) ([]string, error) {
			started <- struct{}{}
			<-release
			return []string{namespace}, nil
		}, func(a, b string) bool { return a < b })
	}))
	namespaces := make([]string, maxNamespaceListConcurrency*2)
	for i := range namespaces {
		namespaces[i] = fmt.Sprintf("ns-%02d", i)
	}
	request := httptest.NewRequest(http.MethodGet, "/?namespace="+strings.Join(namespaces, ","), nil)
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("X-BeaverDeck-Username", "admin")
	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(response, request)
		close(done)
	}()

	for range maxNamespaceListConcurrency {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for namespaced list workers")
		}
	}
	select {
	case <-started:
		t.Fatal("namespaced list exceeded its concurrency limit")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	<-done
	if response.Code != http.StatusOK {
		t.Fatalf("returned status %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
}

func TestCustomResourceReferencesUseCRDPermissions(t *testing.T) {
	const ref = "customresource:widgets.example.io"
	if got := kindToResource(ref); got != "crds" {
		t.Fatalf("kindToResource(%q) = %q, want crds", ref, got)
	}
	if resource, namespaced, err := permissionDeleteTarget(ref); err != nil || resource != "crds" || namespaced {
		t.Fatalf("permissionDeleteTarget(%q) = %q, %t, %v", ref, resource, namespaced, err)
	}
	if name, ok := customResourceCRDName(ref); !ok || name != "widgets.example.io" {
		t.Fatalf("customResourceCRDName(%q) = %q, %t", ref, name, ok)
	}
}

func TestSecretListPermissionCannotReadManifest(t *testing.T) {
	ctx := context.Background()
	store, err := users.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := store.ResetToEmptyConfig(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteBootstrap(ctx, "admin", "admin-pass"); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateRole(ctx, "secret-lister", "viewer", []byte(`{"namespaces":["apps"],"resources":{"secrets":{"view":true}}}`)); err != nil {
		t.Fatal(err)
	}
	if err := store.Create(ctx, "operator", "operator-pass", users.Role("secret-lister")); err != nil {
		t.Fatal(err)
	}
	token, err := store.CreateSession(ctx, "operator", "local")
	if err != nil {
		t.Fatal(err)
	}

	server := New(config.Config{ManagedNamespace: "apps", AllowAllNamespaces: true}, nil, store, embed.FS{})
	handler := auth.Middleware(store)(server.Routes())

	for _, path := range []string{
		"/api/manifest?namespace=apps&kind=secret&name=app-secret",
		"/api/manifest?namespace=apps&kind=secret&name=app-secret&reveal=1",
	} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("Authorization", "Bearer "+token)
		request.Header.Set("X-BeaverDeck-Username", "operator")
		response := httptest.NewRecorder()

		handler.ServeHTTP(response, request)

		if response.Code != http.StatusForbidden {
			t.Fatalf("%s returned status %d, want %d: %s", path, response.Code, http.StatusForbidden, response.Body.String())
		}
		if !strings.Contains(response.Body.String(), "permission denied: edit secrets") {
			t.Fatalf("%s returned unexpected denial: %s", path, response.Body.String())
		}
	}
}

func TestRestartDiagnosticRejectsForbiddenNamespaceBeforeSecretLookup(t *testing.T) {
	ctx := context.Background()
	store, err := users.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.ResetToEmptyConfig(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteBootstrap(ctx, "admin", "admin-pass"); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateRole(ctx, "pod-reader", "viewer", []byte(`{"namespaces":["apps"],"resources":{"pods":{"view":true}}}`)); err != nil {
		t.Fatal(err)
	}
	if err := store.Create(ctx, "operator", "operator-pass", users.Role("pod-reader")); err != nil {
		t.Fatal(err)
	}
	token, err := store.CreateSession(ctx, "operator", "local")
	if err != nil {
		t.Fatal(err)
	}

	server := New(config.Config{ManagedNamespace: "apps", AllowAllNamespaces: true}, nil, store, embed.FS{})
	handler := auth.Middleware(store)(server.Routes())
	request := httptest.NewRequest(http.MethodGet, "/api/restart-diagnostics/example?namespace=restricted", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("X-BeaverDeck-Username", "operator")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("returned status %d, want %d: %s", response.Code, http.StatusForbidden, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "namespace is not allowed") {
		t.Fatalf("returned unexpected denial: %s", response.Body.String())
	}
}

func TestCustomResourceListRejectsForbiddenNamespaceBeforeClusterLookup(t *testing.T) {
	ctx := context.Background()
	store, err := users.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := store.ResetToEmptyConfig(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteBootstrap(ctx, "admin", "admin-pass"); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateRole(ctx, "crd-reader", "viewer", []byte(`{"namespaces":["apps"],"resources":{"crds":{"view":true}}}`)); err != nil {
		t.Fatal(err)
	}
	if err := store.Create(ctx, "operator", "operator-pass", users.Role("crd-reader")); err != nil {
		t.Fatal(err)
	}
	token, err := store.CreateSession(ctx, "operator", "local")
	if err != nil {
		t.Fatal(err)
	}

	server := New(config.Config{ManagedNamespace: "apps", AllowAllNamespaces: true}, nil, store, embed.FS{})
	handler := auth.Middleware(store)(server.Routes())
	request := httptest.NewRequest(http.MethodGet, "/api/crds/widgets.example.io/resources?namespace=restricted", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("X-BeaverDeck-Username", "operator")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("returned status %d, want %d: %s", response.Code, http.StatusForbidden, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "namespace is not allowed") {
		t.Fatalf("returned unexpected denial: %s", response.Body.String())
	}
}

func TestHelmHistoryRejectsForbiddenNamespaceBeforeClusterLookup(t *testing.T) {
	ctx := context.Background()
	store, err := users.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := store.ResetToEmptyConfig(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteBootstrap(ctx, "admin", "admin-pass"); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateRole(ctx, "helm-reader", "viewer", []byte(`{"namespaces":["apps"],"resources":{"applications":{"view":true}}}`)); err != nil {
		t.Fatal(err)
	}
	if err := store.Create(ctx, "operator", "operator-pass", users.Role("helm-reader")); err != nil {
		t.Fatal(err)
	}
	token, err := store.CreateSession(ctx, "operator", "local")
	if err != nil {
		t.Fatal(err)
	}

	server := New(config.Config{ManagedNamespace: "apps", AllowAllNamespaces: true}, nil, store, embed.FS{})
	handler := auth.Middleware(store)(server.Routes())
	request := httptest.NewRequest(http.MethodGet, "/api/helm/releases/demo/history?namespace=restricted", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("X-BeaverDeck-Username", "operator")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("returned status %d, want %d: %s", response.Code, http.StatusForbidden, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "namespace is not allowed") {
		t.Fatalf("returned unexpected denial: %s", response.Body.String())
	}
}

func TestHelmRevisionDetailsRequireManagePermission(t *testing.T) {
	ctx := context.Background()
	store, err := users.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := store.ResetToEmptyConfig(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteBootstrap(ctx, "admin", "admin-pass"); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateRole(ctx, "helm-reader", "viewer", []byte(`{"namespaces":["apps"],"resources":{"applications":{"view":true}}}`)); err != nil {
		t.Fatal(err)
	}
	if err := store.Create(ctx, "operator", "operator-pass", users.Role("helm-reader")); err != nil {
		t.Fatal(err)
	}
	token, err := store.CreateSession(ctx, "operator", "local")
	if err != nil {
		t.Fatal(err)
	}

	server := New(config.Config{ManagedNamespace: "apps", AllowAllNamespaces: true}, nil, store, embed.FS{})
	handler := auth.Middleware(store)(server.Routes())
	request := httptest.NewRequest(http.MethodGet, "/api/helm/releases/demo/revisions/2/values?namespace=apps", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("X-BeaverDeck-Username", "operator")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("returned status %d, want %d: %s", response.Code, http.StatusForbidden, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "permission denied: edit applications") {
		t.Fatalf("returned unexpected denial: %s", response.Body.String())
	}
}

func TestArgoCDHistoryRejectsForbiddenNamespaceBeforeClusterLookup(t *testing.T) {
	ctx := context.Background()
	store, err := users.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := store.ResetToEmptyConfig(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteBootstrap(ctx, "admin", "admin-pass"); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateRole(ctx, "applications-reader", "viewer", []byte(`{"namespaces":["apps"],"resources":{"applications":{"view":true}}}`)); err != nil {
		t.Fatal(err)
	}
	if err := store.Create(ctx, "operator", "operator-pass", users.Role("applications-reader")); err != nil {
		t.Fatal(err)
	}
	token, err := store.CreateSession(ctx, "operator", "local")
	if err != nil {
		t.Fatal(err)
	}

	server := New(config.Config{ManagedNamespace: "apps", AllowAllNamespaces: true}, nil, store, embed.FS{})
	handler := auth.Middleware(store)(server.Routes())
	request := httptest.NewRequest(http.MethodGet, "/api/argocd/applications/demo/history?namespace=restricted", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("X-BeaverDeck-Username", "operator")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("returned status %d, want %d: %s", response.Code, http.StatusForbidden, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "namespace is not allowed") {
		t.Fatalf("returned unexpected denial: %s", response.Body.String())
	}
}

func TestArgoCDRevisionDetailsRequireManagePermission(t *testing.T) {
	ctx := context.Background()
	store, err := users.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := store.ResetToEmptyConfig(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteBootstrap(ctx, "admin", "admin-pass"); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateRole(ctx, "applications-reader", "viewer", []byte(`{"namespaces":["apps"],"resources":{"applications":{"view":true}}}`)); err != nil {
		t.Fatal(err)
	}
	if err := store.Create(ctx, "operator", "operator-pass", users.Role("applications-reader")); err != nil {
		t.Fatal(err)
	}
	token, err := store.CreateSession(ctx, "operator", "local")
	if err != nil {
		t.Fatal(err)
	}

	server := New(config.Config{ManagedNamespace: "apps", AllowAllNamespaces: true}, nil, store, embed.FS{})
	handler := auth.Middleware(store)(server.Routes())
	request := httptest.NewRequest(http.MethodGet, "/api/argocd/applications/demo/revisions/1/source?namespace=apps", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("X-BeaverDeck-Username", "operator")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("returned status %d, want %d: %s", response.Code, http.StatusForbidden, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "permission denied: edit applications") {
		t.Fatalf("returned unexpected denial: %s", response.Body.String())
	}
}
