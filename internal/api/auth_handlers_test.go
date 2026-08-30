package api

import (
	"context"
	"embed"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"beaverdeck/internal/config"
	"beaverdeck/internal/users"
)

func TestAuthProvidersExposeOIDCAndEntraIndependently(t *testing.T) {
	ctx := context.Background()
	store, err := users.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.CompleteBootstrap(ctx, "admin", "admin-pass"); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateOIDCConfig(ctx, users.OIDCConfig{
		ProviderName: "Corporate OIDC",
		IssuerURL:    "https://id.example.com",
		ClientID:     "oidc-client",
		ClientSecret: "oidc-secret",
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateEntraConfig(ctx, users.OIDCConfig{
		ProviderName: "Company Entra",
		IssuerURL:    "https://login.microsoftonline.com/tenant/v2.0",
		ClientID:     "entra-client",
		ClientSecret: "entra-secret",
	}); err != nil {
		t.Fatal(err)
	}

	server := New(config.Config{AppVersion: "test"}, nil, store, embed.FS{})
	request := httptest.NewRequest(http.MethodGet, "/api/auth/providers", nil)
	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("returned status %d: %s", response.Code, response.Body.String())
	}
	var payload struct {
		OIDC struct {
			Enabled      bool   `json:"enabled"`
			ProviderName string `json:"provider_name"`
		} `json:"oidc"`
		Entra struct {
			Enabled      bool   `json:"enabled"`
			ProviderName string `json:"provider_name"`
		} `json:"entra"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.OIDC.Enabled || payload.OIDC.ProviderName != "Corporate OIDC" {
		t.Fatalf("unexpected OIDC provider: %#v", payload.OIDC)
	}
	if !payload.Entra.Enabled || payload.Entra.ProviderName != "Company Entra" {
		t.Fatalf("unexpected Entra provider: %#v", payload.Entra)
	}
}

func TestOAuthStateCookieSelectsOnlyMatchingProvider(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/callback?state=entra-state&code=test", nil)
	request.AddCookie(&http.Cookie{Name: oidcAuthStateCookie, Value: "oidc-state"})
	request.AddCookie(&http.Cookie{Name: entraAuthStateCookie, Value: "entra-state"})

	if !oauthStateCookieMatches(request, entraAuthStateCookie) {
		t.Fatal("expected Entra state cookie to match")
	}
	if oauthStateCookieMatches(request, oidcAuthStateCookie) {
		t.Fatal("generic OIDC state cookie must not match the Entra callback state")
	}
}
