package api

import (
	"context"
	"embed"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"beaverdeck/internal/auth"
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

func TestAuthLogoutRevokesTokenForSubsequentAuthenticatedRequests(t *testing.T) {
	ctx := context.Background()
	store, err := users.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.CompleteBootstrap(ctx, "admin", "admin-pass"); err != nil {
		t.Fatal(err)
	}
	token, err := store.CreateSession(ctx, "admin", "local")
	if err != nil {
		t.Fatal(err)
	}

	server := New(config.Config{AppVersion: "test"}, nil, store, embed.FS{})
	routes := server.Routes()
	secured := auth.Middleware(store)(routes)

	preRequest := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	preRequest.Header.Set("Authorization", "Bearer "+token)
	preRequest.Header.Set("X-BeaverDeck-Username", "admin")
	preResponse := httptest.NewRecorder()
	secured.ServeHTTP(preResponse, preRequest)
	if preResponse.Code != http.StatusOK {
		t.Fatalf("expected token to work before logout, got %d: %s", preResponse.Code, preResponse.Body.String())
	}

	logoutRequest := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logoutRequest.Header.Set("Authorization", "Bearer "+token)
	logoutResponse := httptest.NewRecorder()
	routes.ServeHTTP(logoutResponse, logoutRequest)
	if logoutResponse.Code != http.StatusOK {
		t.Fatalf("expected logout to succeed, got %d: %s", logoutResponse.Code, logoutResponse.Body.String())
	}

	postRequest := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	postRequest.Header.Set("Authorization", "Bearer "+token)
	postRequest.Header.Set("X-BeaverDeck-Username", "admin")
	postResponse := httptest.NewRecorder()
	secured.ServeHTTP(postResponse, postRequest)
	if postResponse.Code != http.StatusUnauthorized {
		t.Fatalf("expected token to be rejected after logout, got %d: %s", postResponse.Code, postResponse.Body.String())
	}
}

func TestAuthLogoutWithAlreadyInvalidTokenIsSilentSuccess(t *testing.T) {
	store, err := users.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	server := New(config.Config{AppVersion: "test"}, nil, store, embed.FS{})
	routes := server.Routes()

	logoutRequest := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logoutRequest.Header.Set("Authorization", "Bearer not-a-real-token")
	logoutResponse := httptest.NewRecorder()
	routes.ServeHTTP(logoutResponse, logoutRequest)
	if logoutResponse.Code != http.StatusOK {
		t.Fatalf("expected logout with an already-invalid token to succeed quietly, got %d: %s", logoutResponse.Code, logoutResponse.Body.String())
	}
}

func TestAuthLogoutIgnoresTokenInFormBodyOnlyHeaderIsHonored(t *testing.T) {
	ctx := context.Background()
	store, err := users.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.CompleteBootstrap(ctx, "admin", "admin-pass"); err != nil {
		t.Fatal(err)
	}
	token, err := store.CreateSession(ctx, "admin", "local")
	if err != nil {
		t.Fatal(err)
	}

	server := New(config.Config{AppVersion: "test"}, nil, store, embed.FS{})
	routes := server.Routes()
	secured := auth.Middleware(store)(routes)

	body := url.Values{"token": {token}}.Encode()
	logoutRequest := httptest.NewRequest(http.MethodPost, "/api/auth/logout", strings.NewReader(body))
	logoutRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	logoutResponse := httptest.NewRecorder()
	routes.ServeHTTP(logoutResponse, logoutRequest)
	if logoutResponse.Code != http.StatusOK {
		t.Fatalf("expected logout with no Authorization header to still respond OK, got %d: %s", logoutResponse.Code, logoutResponse.Body.String())
	}

	postRequest := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	postRequest.Header.Set("Authorization", "Bearer "+token)
	postRequest.Header.Set("X-BeaverDeck-Username", "admin")
	postResponse := httptest.NewRecorder()
	secured.ServeHTTP(postResponse, postRequest)
	if postResponse.Code != http.StatusOK {
		t.Fatalf("expected token sent only via form body to remain valid, got %d: %s", postResponse.Code, postResponse.Body.String())
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
