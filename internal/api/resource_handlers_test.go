package api

import (
	"context"
	"embed"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
