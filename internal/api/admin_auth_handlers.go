package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"beaverdeck/internal/users"
)

func (s *Server) adminGoogleConfigGet(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	cfg, err := s.users.GetGoogleConfig(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (s *Server) adminGoogleConfigUpdate(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var req users.GoogleConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := s.users.UpdateGoogleConfig(r.Context(), req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) adminGoogleConfigTest(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var req users.GoogleConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	redirectURI := googleRedirectURI(r)
	if strings.TrimSpace(req.ClientID) == "" || strings.TrimSpace(req.ClientSecret) == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("google client id/client secret are not configured"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	if err := probeOAuthClientCredentials(ctx, "https://oauth2.googleapis.com/token", req.ClientID, req.ClientSecret, redirectURI); err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("google oauth client validation failed: %w", err))
		return
	}
	groups, err := fetchGoogleGroups(ctx, req, req.DelegatedAdminEmail)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":                "ok",
		"oauth_client":          true,
		"directory_api":         true,
		"delegated_admin":       strings.TrimSpace(strings.ToLower(req.DelegatedAdminEmail)),
		"delegated_group_count": len(groups),
		"redirect_uri":          redirectURI,
		"message":               fmt.Sprintf("Google config is valid. OAuth client credentials were accepted for redirect URI %s, and Directory API returned %d groups for %s.", redirectURI, len(groups), strings.TrimSpace(strings.ToLower(req.DelegatedAdminEmail))),
	})
}

func (s *Server) adminGoogleMappingsList(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	items, err := s.users.ListGoogleGroupRoles(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) adminGoogleReset(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	if err := s.users.ResetGoogleAuth(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) adminGoogleMappingsUpsert(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var req googleMappingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := s.users.UpsertGoogleGroupRole(r.Context(), req.GroupEmail, users.Role(req.Role)); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) adminGoogleMappingsDelete(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var req googleMappingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := s.users.DeleteGoogleGroupRole(r.Context(), req.GroupEmail); err != nil {
		if err == sql.ErrNoRows {
			writeErr(w, http.StatusNotFound, err)
			return
		}
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) adminOIDCConfigGet(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	cfg, err := s.users.GetOIDCConfig(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (s *Server) adminOIDCConfigUpdate(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var req users.OIDCConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := s.users.UpdateOIDCConfig(r.Context(), req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) adminOIDCConfigTest(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var req users.OIDCConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(req.IssuerURL) == "" || strings.TrimSpace(req.ClientID) == "" || strings.TrimSpace(req.ClientSecret) == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("OpenID Connect issuer/client id/client secret are not configured"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	discovery, err := fetchOIDCDiscovery(ctx, req.IssuerURL)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := probeOAuthClientCredentials(ctx, discovery.TokenEndpoint, req.ClientID, req.ClientSecret, oidcRedirectURI(r)); err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("OpenID Connect client validation failed: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":        "ok",
		"provider_name": providerLabel(req.ProviderName, "OpenID Connect"),
		"issuer_url":    strings.TrimSpace(req.IssuerURL),
		"redirect_uri":  oidcRedirectURI(r),
		"scopes":        oidcScopes(req.Scopes),
		"userinfo_url":  discovery.UserInfoEndpoint,
		"entra_graph":   isMicrosoftEntraConfig(req),
		"message":       fmt.Sprintf("%s config is valid. Discovery succeeded for %s, token endpoint accepted the client credentials, and redirect URI is %s.", providerLabel(req.ProviderName, "OpenID Connect"), strings.TrimSpace(req.IssuerURL), oidcRedirectURI(r)),
	})
}

func (s *Server) adminOIDCReset(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	if err := s.users.ResetOIDCAuth(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) adminOIDCMappingsList(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	items, err := s.users.ListOIDCGroupRoles(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) adminOIDCMappingsUpsert(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var req oidcMappingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := s.users.UpsertOIDCGroupRole(r.Context(), req.GroupName, users.Role(req.Role)); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) adminOIDCMappingsDelete(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var req oidcMappingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := s.users.DeleteOIDCGroupRole(r.Context(), req.GroupName); err != nil {
		if err == sql.ErrNoRows {
			writeErr(w, http.StatusNotFound, err)
			return
		}
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) adminUsersRevokeSessions(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var req revokeSessionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := s.users.InvalidateUserSessions(r.Context(), req.Username); err != nil {
		if err == sql.ErrNoRows {
			writeErr(w, http.StatusNotFound, err)
			return
		}
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) adminUsersResetPassword(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var req resetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := s.users.ResetLocalPassword(r.Context(), req.Username, req.Password); err != nil {
		if err == sql.ErrNoRows {
			writeErr(w, http.StatusNotFound, err)
			return
		}
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}
