package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func (s *Server) authProviders(w http.ResponseWriter, r *http.Request) {
	bootstrapStatus, err := s.users.GetBootstrapStatus(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	googleCfg, err := s.users.GetGoogleConfig(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	oidcCfg, err := s.users.GetOIDCConfig(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"initialized": bootstrapStatus.Initialized,
		"local":       bootstrapStatus.Initialized,
		"appVersion":  s.cfg.AppVersion,
		"google": map[string]any{
			"enabled": bootstrapStatus.Initialized &&
				strings.TrimSpace(googleCfg.ClientID) != "" &&
				strings.TrimSpace(googleCfg.ClientSecret) != "" &&
				strings.TrimSpace(googleCfg.ServiceAccountJSON) != "" &&
				strings.TrimSpace(googleCfg.DelegatedAdminEmail) != "",
			"hosted_domain": googleCfg.HostedDomain,
		},
		"oidc": map[string]any{
			"enabled": bootstrapStatus.Initialized &&
				strings.TrimSpace(oidcCfg.IssuerURL) != "" &&
				strings.TrimSpace(oidcCfg.ClientID) != "" &&
				strings.TrimSpace(oidcCfg.ClientSecret) != "",
			"provider_name": providerLabel(oidcCfg.ProviderName, "Custom OAuth"),
			"hosted_domain": oidcCfg.HostedDomain,
		},
	})
}

type bootstrapCompleteRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

func (s *Server) authBootstrapStatus(w http.ResponseWriter, r *http.Request) {
	status, err := s.users.GetBootstrapStatus(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"initialized": status.Initialized,
	})
}

func (s *Server) authBootstrapComplete(w http.ResponseWriter, r *http.Request) {
	var req bootstrapCompleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := s.users.CompleteBootstrap(r.Context(), req.Token, req.Password); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "already initialized") {
			writeErr(w, http.StatusConflict, err)
			return
		}
		if strings.Contains(strings.ToLower(err.Error()), "invalid bootstrap token") {
			writeErr(w, http.StatusUnauthorized, err)
			return
		}
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) authLogin(w http.ResponseWriter, r *http.Request) {
	bootstrapStatus, err := s.users.GetBootstrapStatus(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if !bootstrapStatus.Initialized {
		writeErr(w, http.StatusConflict, fmt.Errorf("application is not initialized"))
		return
	}
	var req localLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	user, err := s.users.VerifyLocalCredentials(r.Context(), req.Username, req.Password)
	if err != nil {
		if err == sql.ErrNoRows {
			writeErr(w, http.StatusUnauthorized, fmt.Errorf("invalid username or password"))
			return
		}
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	sessionToken, err := s.users.CreateSession(r.Context(), user.Username, user.AuthSource)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"username": user.Username,
		"token":    sessionToken,
	})
}

func (s *Server) authLogout(w http.ResponseWriter, r *http.Request) {
	token := requestBearerToken(r)
	if token != "" {
		if err := s.users.RevokeSession(r.Context(), token); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) authGoogleStart(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.users.GetGoogleConfig(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if strings.TrimSpace(cfg.ClientID) == "" || strings.TrimSpace(cfg.ClientSecret) == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("google auth is not configured"))
		return
	}

	state, err := randomStateToken()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	setOAuthStateCookie(w, googleAuthStateCookie, state, requestIsSecure(r), 600)

	params := url.Values{}
	params.Set("client_id", cfg.ClientID)
	params.Set("redirect_uri", googleRedirectURI(r))
	params.Set("response_type", "code")
	params.Set("scope", "openid email profile")
	params.Set("state", state)
	params.Set("prompt", "select_account")
	if strings.TrimSpace(cfg.HostedDomain) != "" {
		params.Set("hd", strings.TrimSpace(cfg.HostedDomain))
	}
	http.Redirect(w, r, "https://accounts.google.com/o/oauth2/v2/auth?"+params.Encode(), http.StatusFound)
}

func (s *Server) authGoogleCallback(w http.ResponseWriter, r *http.Request) {
	defer clearOAuthStateCookie(w, googleAuthStateCookie, requestIsSecure(r))

	cfg, err := s.users.GetGoogleConfig(r.Context())
	if err != nil {
		s.redirectAuthResult(w, r, "", "", err)
		return
	}

	code, err := validateOAuthCallback(r, googleAuthStateCookie)
	if err != nil {
		s.redirectAuthResult(w, r, "", "", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	accessToken, err := exchangeGoogleCode(ctx, cfg, googleRedirectURI(r), code)
	if err != nil {
		s.redirectAuthResult(w, r, "", "", err)
		return
	}
	profile, err := fetchGoogleUserInfo(ctx, accessToken)
	if err != nil {
		s.redirectAuthResult(w, r, "", "", err)
		return
	}
	if !profile.EmailVerified {
		s.redirectAuthResult(w, r, "", "", fmt.Errorf("google account email is not verified"))
		return
	}

	email := strings.TrimSpace(strings.ToLower(profile.Email))
	if email == "" || strings.TrimSpace(profile.Sub) == "" {
		s.redirectAuthResult(w, r, "", "", fmt.Errorf("google account is missing email identity"))
		return
	}
	if hosted := strings.TrimSpace(strings.ToLower(cfg.HostedDomain)); hosted != "" && !strings.HasSuffix(email, "@"+hosted) {
		s.redirectAuthResult(w, r, "", "", fmt.Errorf("google account %s is outside the allowed hosted domain", email))
		return
	}

	groups, err := fetchGoogleGroups(ctx, cfg, email)
	if err != nil {
		s.redirectAuthResult(w, r, "", "", err)
		return
	}
	role, _, err := s.users.ResolveGoogleRole(ctx, groups)
	if err != nil {
		s.redirectAuthResult(w, r, "", "", err)
		return
	}
	if err := s.users.UpsertGoogleUser(ctx, email, strings.TrimSpace(profile.Sub), role); err != nil {
		s.redirectAuthResult(w, r, "", "", err)
		return
	}
	sessionToken, err := s.users.CreateSession(ctx, email, "google")
	if err != nil {
		s.redirectAuthResult(w, r, "", "", err)
		return
	}
	s.redirectAuthResult(w, r, email, sessionToken, nil)
}

func (s *Server) authOIDCStart(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.users.GetOIDCConfig(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if strings.TrimSpace(cfg.IssuerURL) == "" || strings.TrimSpace(cfg.ClientID) == "" || strings.TrimSpace(cfg.ClientSecret) == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("custom oauth is not configured"))
		return
	}

	discovery, err := fetchOIDCDiscovery(r.Context(), cfg.IssuerURL)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	state, err := randomStateToken()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	setOAuthStateCookie(w, oidcAuthStateCookie, state, requestIsSecure(r), 600)

	params := url.Values{}
	params.Set("client_id", cfg.ClientID)
	params.Set("redirect_uri", oidcRedirectURI(r))
	params.Set("response_type", "code")
	params.Set("scope", oidcScopes(cfg.Scopes))
	params.Set("state", state)
	http.Redirect(w, r, discovery.AuthorizationEndpoint+"?"+params.Encode(), http.StatusFound)
}

func (s *Server) authOIDCCallback(w http.ResponseWriter, r *http.Request) {
	defer clearOAuthStateCookie(w, oidcAuthStateCookie, requestIsSecure(r))

	cfg, err := s.users.GetOIDCConfig(r.Context())
	if err != nil {
		s.redirectAuthResult(w, r, "", "", err)
		return
	}

	code, err := validateOAuthCallback(r, oidcAuthStateCookie)
	if err != nil {
		s.redirectAuthResult(w, r, "", "", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	discovery, err := fetchOIDCDiscovery(ctx, cfg.IssuerURL)
	if err != nil {
		s.redirectAuthResult(w, r, "", "", err)
		return
	}
	accessToken, err := exchangeOAuthCode(ctx, discovery.TokenEndpoint, cfg.ClientID, cfg.ClientSecret, oidcRedirectURI(r), code)
	if err != nil {
		s.redirectAuthResult(w, r, "", "", err)
		return
	}
	userInfo, err := fetchOIDCUserInfo(ctx, discovery.UserInfoEndpoint, accessToken)
	if err != nil {
		s.redirectAuthResult(w, r, "", "", err)
		return
	}
	email, err := extractStringClaim(userInfo, cfg.EmailClaim, "email")
	if err != nil {
		s.redirectAuthResult(w, r, "", "", err)
		return
	}
	subject, err := extractStringClaim(userInfo, "sub", "sub")
	if err != nil {
		s.redirectAuthResult(w, r, "", "", err)
		return
	}
	email = strings.TrimSpace(strings.ToLower(email))
	if hosted := strings.TrimSpace(strings.ToLower(cfg.HostedDomain)); hosted != "" && !strings.HasSuffix(email, "@"+hosted) {
		s.redirectAuthResult(w, r, "", "", fmt.Errorf("custom oauth account %s is outside the allowed hosted domain", email))
		return
	}

	groups, err := extractStringListClaim(userInfo, cfg.GroupsClaim, "groups")
	if err != nil {
		s.redirectAuthResult(w, r, "", "", err)
		return
	}
	role, _, err := s.users.ResolveOIDCRole(ctx, groups)
	if err != nil {
		s.redirectAuthResult(w, r, "", "", err)
		return
	}
	if err := s.users.UpsertOIDCUser(ctx, email, subject, role); err != nil {
		s.redirectAuthResult(w, r, "", "", err)
		return
	}
	sessionToken, err := s.users.CreateSession(ctx, email, "oidc")
	if err != nil {
		s.redirectAuthResult(w, r, "", "", err)
		return
	}
	s.redirectAuthResult(w, r, email, sessionToken, nil)
}

func (s *Server) redirectAuthResult(w http.ResponseWriter, r *http.Request, username, token string, err error) {
	values := url.Values{}
	if err != nil {
		values.Set("auth_error", errString(err))
	} else {
		values.Set("auth_user", username)
		values.Set("auth_session", token)
	}
	basePath := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Prefix"), ",")[0])
	if basePath != "" && basePath != "/" {
		basePath = "/" + strings.Trim(basePath, "/")
	} else {
		basePath = ""
	}
	http.Redirect(w, r, basePath+"/#"+values.Encode(), http.StatusFound)
}
