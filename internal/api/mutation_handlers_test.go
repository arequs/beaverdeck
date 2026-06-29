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

func TestExistingManifestEditDoesNotRequireApplyPermission(t *testing.T) {
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
	if err := store.CreateRole(ctx, "workload-editor", "viewer", []byte(`{"namespaces":["apps"],"resources":{"workloads":{"edit":true}}}`)); err != nil {
		t.Fatal(err)
	}
	if err := store.Create(ctx, "operator", "operator-pass", users.Role("workload-editor")); err != nil {
		t.Fatal(err)
	}
	token, err := store.CreateSession(ctx, "operator", "local")
	if err != nil {
		t.Fatal(err)
	}

	server := New(config.Config{ManagedNamespace: "apps", AllowAllNamespaces: true}, nil, store, embed.FS{})
	handler := auth.Middleware(store)(server.Routes())

	editBody := `{"namespace":"apps","kind":"deployment","name":"expected","yaml":"apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: different\n","dryRun":true}`
	editRequest := httptest.NewRequest(http.MethodPut, "/api/manifest", strings.NewReader(editBody))
	editRequest.Header.Set("Authorization", "Bearer "+token)
	editRequest.Header.Set("X-BeaverDeck-Username", "operator")
	editResponse := httptest.NewRecorder()
	handler.ServeHTTP(editResponse, editRequest)

	if editResponse.Code == http.StatusForbidden {
		t.Fatalf("resource editor was denied by Apply YAML permission: %s", editResponse.Body.String())
	}
	if editResponse.Code != http.StatusInternalServerError || !strings.Contains(editResponse.Body.String(), "does not match") {
		t.Fatalf("expected request to reach manifest validation, got status %d: %s", editResponse.Code, editResponse.Body.String())
	}

	applyRequest := httptest.NewRequest(http.MethodPost, "/api/apply", strings.NewReader(`{}`))
	applyRequest.Header.Set("Authorization", "Bearer "+token)
	applyRequest.Header.Set("X-BeaverDeck-Username", "operator")
	applyResponse := httptest.NewRecorder()
	handler.ServeHTTP(applyResponse, applyRequest)

	if applyResponse.Code != http.StatusForbidden {
		t.Fatalf("Apply YAML should still require apply edit permission, got status %d: %s", applyResponse.Code, applyResponse.Body.String())
	}
}
