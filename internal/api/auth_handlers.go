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
	entraCfg, err := s.users.GetEntraConfig(r.Context())
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
			"provider_name": providerLabel(oidcCfg.ProviderName, "OpenID Connect"),
			"hosted_domain": oidcCfg.HostedDomain,
		},
		"entra": map[string]any{
			"enabled": bootstrapStatus.Initialized &&
				strings.TrimSpace(entraCfg.IssuerURL) != "" &&
				strings.TrimSpace(entraCfg.ClientID) != "" &&
				strings.TrimSpace(entraCfg.ClientSecret) != "",
			"provider_name": providerLabel(entraCfg.ProviderName, "Azure Entra ID"),
			"hosted_domain": entraCfg.HostedDomain,
		},
	})
}

type bootstrapCompleteRequest struct {
	Username string `json:"username"`
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
	if err := s.users.CompleteBootstrap(r.Context(), req.Username, req.Password); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "already initialized") {
			writeErr(w, http.StatusConflict, err)
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
	sessionToken, err := s.users.CreateExternalSession(ctx, email, "google", role)
	if err != nil {
		s.redirectAuthResult(w, r, "", "", err)
		return
	}
	s.redirectAuthResult(w, r, email, sessionToken, nil)
}

func (s *Server) authOIDCStart(w http.ResponseWriter, r *http.Request) {
	s.authExternalOIDCStart(w, r, false)
}

func (s *Server) authEntraStart(w http.ResponseWriter, r *http.Request) {
	s.authExternalOIDCStart(w, r, true)
}

func (s *Server) authExternalOIDCStart(w http.ResponseWriter, r *http.Request, entra bool) {
	cfg, err := s.users.GetOIDCConfig(r.Context())
	providerName := "OpenID Connect"
	stateCookie := oidcAuthStateCookie
	redirectURI := oidcRedirectURI(r)
	if entra {
		cfg, err = s.users.GetEntraConfig(r.Context())
		providerName = "Azure Entra ID"
		stateCookie = entraAuthStateCookie
		redirectURI = entraRedirectURI(r)
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if strings.TrimSpace(cfg.IssuerURL) == "" || strings.TrimSpace(cfg.ClientID) == "" || strings.TrimSpace(cfg.ClientSecret) == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("%s is not configured", providerName))
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
	setOAuthStateCookie(w, stateCookie, state, requestIsSecure(r), 600)

	params := url.Values{}
	params.Set("client_id", cfg.ClientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("response_type", "code")
	params.Set("scope", oidcScopes(cfg.Scopes))
	params.Set("state", state)
	http.Redirect(w, r, discovery.AuthorizationEndpoint+"?"+params.Encode(), http.StatusFound)
}

func (s *Server) authOIDCCallback(w http.ResponseWriter, r *http.Request) {
	if oauthStateCookieMatches(r, entraAuthStateCookie) {
		s.authExternalOIDCCallback(w, r, true)
		return
	}
	s.authExternalOIDCCallback(w, r, false)
}

func (s *Server) authExternalOIDCCallback(w http.ResponseWriter, r *http.Request, entra bool) {
	stateCookie := oidcAuthStateCookie
	redirectURI := oidcRedirectURI(r)
	providerName := "OpenID Connect"
	authSource := "oidc"
	if entra {
		stateCookie = entraAuthStateCookie
		redirectURI = entraRedirectURI(r)
		providerName = "Azure Entra ID"
		authSource = "entra"
	}
	defer clearOAuthStateCookie(w, stateCookie, requestIsSecure(r))

	cfg, err := s.users.GetOIDCConfig(r.Context())
	if entra {
		cfg, err = s.users.GetEntraConfig(r.Context())
	}
	if err != nil {
		s.redirectAuthResult(w, r, "", "", err)
		return
	}

	code, err := validateOAuthCallback(r, stateCookie)
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
	tokenResp, _, err := executeOAuthCodeExchange(ctx, discovery.TokenEndpoint, cfg.ClientID, cfg.ClientSecret, redirectURI, code)
	if err != nil {
		s.redirectAuthResult(w, r, "", "", err)
		return
	}
	accessToken := strings.TrimSpace(tokenResp.AccessToken)
	if accessToken == "" {
		s.redirectAuthResult(w, r, "", "", fmt.Errorf("oauth token response did not include an access token"))
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
	if strings.TrimSpace(subject) == "" {
		s.redirectAuthResult(w, r, "", "", fmt.Errorf("%s account is missing subject identity", providerName))
		return
	}
	email = strings.TrimSpace(strings.ToLower(email))
	if hosted := strings.TrimSpace(strings.ToLower(cfg.HostedDomain)); hosted != "" && !strings.HasSuffix(email, "@"+hosted) {
		s.redirectAuthResult(w, r, "", "", fmt.Errorf("%s account %s is outside the allowed hosted domain", providerName, email))
		return
	}

	groups, groupsErr := extractStringListClaim(userInfo, cfg.GroupsClaim, "groups")
	if entra {
		graphGroups, err := fetchMicrosoftGraphGroups(ctx, accessToken)
		if err != nil && groupsErr != nil {
			s.redirectAuthResult(w, r, "", "", fmt.Errorf("%v; microsoft graph group lookup also failed: %w", groupsErr, err))
			return
		}
		if err == nil {
			groups = appendUniqueStrings(groups, graphGroups)
		}
	}
	if len(groups) == 0 {
		if groupsErr != nil {
			s.redirectAuthResult(w, r, "", "", groupsErr)
			return
		}
		s.redirectAuthResult(w, r, "", "", fmt.Errorf("%s user is not a member of any groups", providerName))
		return
	}
	role, _, err := s.users.ResolveOIDCRole(ctx, groups)
	if entra {
		role, _, err = s.users.ResolveEntraRole(ctx, groups)
	}
	if err != nil {
		s.redirectAuthResult(w, r, "", "", err)
		return
	}
	sessionToken, err := s.users.CreateExternalSession(ctx, email, authSource, role)
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
