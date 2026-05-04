package api

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"beaverdeck/internal/users"
	"golang.org/x/oauth2/jwt"
)

const (
	googleAuthStateCookie = "beaverdeck_google_oauth_state"
	oidcAuthStateCookie   = "beaverdeck_oidc_oauth_state"
)

type localLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type googleGroupsResponse struct {
	Groups        []googleGroupItem `json:"groups"`
	NextPageToken string            `json:"nextPageToken"`
}

type googleGroupItem struct {
	Email string `json:"email"`
}

type googleTokenResponse struct {
	AccessToken      string `json:"access_token"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

type googleUserInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	HostedDomain  string `json:"hd"`
}

type googleServiceAccountKey struct {
	ClientEmail  string `json:"client_email"`
	PrivateKey   string `json:"private_key"`
	PrivateKeyID string `json:"private_key_id"`
	TokenURI     string `json:"token_uri"`
}

type oidcDiscoveryDocument struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserInfoEndpoint      string `json:"userinfo_endpoint"`
}

type googleMappingRequest struct {
	GroupEmail string `json:"group_email"`
	Role       string `json:"role"`
}

type oidcMappingRequest struct {
	GroupName string `json:"group_name"`
	Role      string `json:"role,omitempty"`
}

type revokeSessionsRequest struct {
	Username string `json:"username"`
}

type resetPasswordRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func setOAuthStateCookie(w http.ResponseWriter, name, value string, secure bool, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		MaxAge:   maxAge,
	})
}

func clearOAuthStateCookie(w http.ResponseWriter, name string, secure bool) {
	setOAuthStateCookie(w, name, "", secure, -1)
}

func validateOAuthCallback(r *http.Request, cookieName string) (string, error) {
	stateCookie, err := r.Cookie(cookieName)
	if err != nil || strings.TrimSpace(stateCookie.Value) == "" {
		return "", fmt.Errorf("missing oauth state")
	}
	if !subtleCompare(strings.TrimSpace(r.URL.Query().Get("state")), strings.TrimSpace(stateCookie.Value)) {
		return "", fmt.Errorf("oauth state mismatch")
	}
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if code == "" {
		return "", fmt.Errorf("oauth provider did not return an auth code")
	}
	return code, nil
}

func exchangeGoogleCode(ctx context.Context, cfg users.GoogleConfig, redirectURI, code string) (string, error) {
	return exchangeOAuthCode(ctx, "https://oauth2.googleapis.com/token", cfg.ClientID, cfg.ClientSecret, redirectURI, code)
}

func fetchGoogleUserInfo(ctx context.Context, accessToken string) (googleUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://openidconnect.googleapis.com/v1/userinfo", nil)
	if err != nil {
		return googleUserInfo{}, err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(accessToken))
	var profile googleUserInfo
	if err := doJSON(req, &profile); err != nil {
		return googleUserInfo{}, fmt.Errorf("fetch google user profile: %w", err)
	}
	return profile, nil
}

func fetchOIDCDiscovery(ctx context.Context, issuerURL string) (oidcDiscoveryDocument, error) {
	issuerURL = strings.TrimRight(strings.TrimSpace(issuerURL), "/")
	if issuerURL == "" {
		return oidcDiscoveryDocument{}, fmt.Errorf("custom oauth issuer url is not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, issuerURL+"/.well-known/openid-configuration", nil)
	if err != nil {
		return oidcDiscoveryDocument{}, err
	}
	var doc oidcDiscoveryDocument
	if err := doJSON(req, &doc); err != nil {
		return oidcDiscoveryDocument{}, fmt.Errorf("fetch oidc discovery: %w", err)
	}
	if strings.TrimSpace(doc.AuthorizationEndpoint) == "" || strings.TrimSpace(doc.TokenEndpoint) == "" || strings.TrimSpace(doc.UserInfoEndpoint) == "" {
		return oidcDiscoveryDocument{}, fmt.Errorf("oidc discovery document is missing authorization_endpoint, token_endpoint or userinfo_endpoint")
	}
	return doc, nil
}

func exchangeOAuthCode(ctx context.Context, tokenEndpoint, clientID, clientSecret, redirectURI, code string) (string, error) {
	tokenResp, _, err := executeOAuthCodeExchange(ctx, tokenEndpoint, clientID, clientSecret, redirectURI, code)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(tokenResp.AccessToken) == "" {
		return "", fmt.Errorf("oauth token response did not include an access token")
	}
	return strings.TrimSpace(tokenResp.AccessToken), nil
}

func executeOAuthCodeExchange(ctx context.Context, tokenEndpoint, clientID, clientSecret, redirectURI, code string) (googleTokenResponse, string, error) {
	resp, rawBody, err := oauthTokenRequest(ctx, tokenEndpoint, clientID, clientSecret, redirectURI, code, true)
	if err == nil {
		return resp, rawBody, nil
	}
	if !isInvalidClientError(resp, rawBody) {
		return googleTokenResponse{}, rawBody, fmt.Errorf("exchange oauth auth code: %w", err)
	}
	resp, rawBody, err = oauthTokenRequest(ctx, tokenEndpoint, clientID, clientSecret, redirectURI, code, false)
	if err != nil {
		return googleTokenResponse{}, rawBody, fmt.Errorf("exchange oauth auth code: %w", err)
	}
	return resp, rawBody, nil
}

func probeOAuthClientCredentials(ctx context.Context, tokenEndpoint, clientID, clientSecret, redirectURI string) error {
	const probeCode = "beaverdeck-probe-invalid-code"

	resp, rawBody, err := oauthTokenRequest(ctx, tokenEndpoint, clientID, clientSecret, redirectURI, probeCode, true)
	if err == nil {
		return nil
	}
	if isInvalidClientError(resp, rawBody) {
		resp, rawBody, err = oauthTokenRequest(ctx, tokenEndpoint, clientID, clientSecret, redirectURI, probeCode, false)
		if err == nil {
			return nil
		}
	}
	if isAcceptedProbeError(resp, rawBody) {
		return nil
	}
	if isInvalidClientError(resp, rawBody) {
		return fmt.Errorf("token endpoint rejected client credentials")
	}
	return fmt.Errorf("token endpoint did not accept the configuration: %s", firstNonEmpty(strings.TrimSpace(resp.ErrorDescription), strings.TrimSpace(resp.Error), strings.TrimSpace(rawBody), err.Error()))
}

func oauthTokenRequest(ctx context.Context, tokenEndpoint, clientID, clientSecret, redirectURI, code string, useBasicAuth bool) (googleTokenResponse, string, error) {
	form := url.Values{}
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("grant_type", "authorization_code")
	if !useBasicAuth {
		form.Set("client_id", clientID)
		form.Set("client_secret", clientSecret)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return googleTokenResponse{}, "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if useBasicAuth {
		req.SetBasicAuth(clientID, clientSecret)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	httpResp, err := client.Do(req)
	if err != nil {
		return googleTokenResponse{}, "", err
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(httpResp.Body, 1<<20))
	if err != nil {
		return googleTokenResponse{}, "", err
	}
	rawBody := strings.TrimSpace(string(body))
	var tokenResp googleTokenResponse
	_ = json.Unmarshal(body, &tokenResp)
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return tokenResp, rawBody, fmt.Errorf("%s", firstNonEmpty(strings.TrimSpace(tokenResp.ErrorDescription), strings.TrimSpace(tokenResp.Error), rawBody, httpResp.Status))
	}
	return tokenResp, rawBody, nil
}

func isInvalidClientError(resp googleTokenResponse, raw string) bool {
	value := strings.ToLower(firstNonEmpty(resp.Error, resp.ErrorDescription, raw))
	return strings.Contains(value, "invalid_client") || strings.Contains(value, "client authentication failed")
}

func isAcceptedProbeError(resp googleTokenResponse, raw string) bool {
	value := strings.ToLower(firstNonEmpty(resp.Error, resp.ErrorDescription, raw))
	if value == "" {
		return false
	}
	return strings.Contains(value, "invalid_grant") ||
		strings.Contains(value, "invalid_request") ||
		strings.Contains(value, "invalid code") ||
		strings.Contains(value, "authorization code") ||
		strings.Contains(value, "code verifier") ||
		strings.Contains(value, "pkce") ||
		strings.Contains(value, "unsupported_grant_type")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func fetchOIDCUserInfo(ctx context.Context, userInfoEndpoint, accessToken string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, userInfoEndpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(accessToken))
	var payload map[string]any
	if err := doJSON(req, &payload); err != nil {
		return nil, fmt.Errorf("fetch custom oauth user profile: %w", err)
	}
	return payload, nil
}

func extractStringClaim(payload map[string]any, configured, fallback string) (string, error) {
	keys := []string{strings.TrimSpace(configured), strings.TrimSpace(fallback)}
	seen := map[string]struct{}{}
	for _, key := range keys {
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if value, ok := payload[key]; ok {
			if claim, ok := value.(string); ok && strings.TrimSpace(claim) != "" {
				return strings.TrimSpace(claim), nil
			}
		}
	}
	if strings.TrimSpace(configured) == "" {
		configured = fallback
	}
	return "", fmt.Errorf("required claim is missing: %s", strings.TrimSpace(configured))
}

func extractStringListClaim(payload map[string]any, configured, fallback string) ([]string, error) {
	keys := []string{strings.TrimSpace(configured), strings.TrimSpace(fallback)}
	seen := map[string]struct{}{}
	for _, key := range keys {
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		value, ok := payload[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case []any:
			out := make([]string, 0, len(typed))
			for _, item := range typed {
				if claim, ok := item.(string); ok && strings.TrimSpace(claim) != "" {
					out = append(out, strings.TrimSpace(claim))
				}
			}
			return out, nil
		case []string:
			out := make([]string, 0, len(typed))
			for _, item := range typed {
				if strings.TrimSpace(item) != "" {
					out = append(out, strings.TrimSpace(item))
				}
			}
			return out, nil
		case string:
			if strings.TrimSpace(typed) != "" {
				return []string{strings.TrimSpace(typed)}, nil
			}
		}
	}
	if strings.TrimSpace(configured) == "" {
		configured = fallback
	}
	return nil, fmt.Errorf("required groups claim is missing: %s", strings.TrimSpace(configured))
}

func fetchGoogleGroups(ctx context.Context, cfg users.GoogleConfig, email string) ([]string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return nil, fmt.Errorf("google email is required")
	}
	directoryToken, err := googleDirectoryAccessToken(ctx, cfg)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 15 * time.Second}
	var (
		groups    []string
		pageToken string
	)
	for {
		params := url.Values{}
		params.Set("userKey", email)
		params.Set("maxResults", "200")
		if pageToken != "" {
			params.Set("pageToken", pageToken)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://admin.googleapis.com/admin/directory/v1/groups?"+params.Encode(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+directoryToken)

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetch google groups: %w", err)
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read google groups response: %w", readErr)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("fetch google groups: %s", strings.TrimSpace(string(body)))
		}

		var payload googleGroupsResponse
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, fmt.Errorf("decode google groups response: %w", err)
		}
		for _, item := range payload.Groups {
			if group := strings.TrimSpace(strings.ToLower(item.Email)); group != "" {
				groups = append(groups, group)
			}
		}
		if strings.TrimSpace(payload.NextPageToken) == "" {
			break
		}
		pageToken = strings.TrimSpace(payload.NextPageToken)
	}
	return groups, nil
}

func googleDirectoryAccessToken(ctx context.Context, cfg users.GoogleConfig) (string, error) {
	serviceAccountJSON := strings.TrimSpace(cfg.ServiceAccountJSON)
	if serviceAccountJSON == "" {
		return "", fmt.Errorf("google service account json is not configured")
	}
	delegatedAdminEmail := strings.TrimSpace(strings.ToLower(cfg.DelegatedAdminEmail))
	if delegatedAdminEmail == "" {
		return "", fmt.Errorf("google delegated admin email is not configured")
	}

	var key googleServiceAccountKey
	if err := json.Unmarshal([]byte(serviceAccountJSON), &key); err != nil {
		return "", fmt.Errorf("decode google service account json: %w", err)
	}
	if strings.TrimSpace(key.ClientEmail) == "" || strings.TrimSpace(key.PrivateKey) == "" {
		return "", fmt.Errorf("google service account json is missing client_email or private_key")
	}
	tokenURL := strings.TrimSpace(key.TokenURI)
	if tokenURL == "" {
		tokenURL = "https://oauth2.googleapis.com/token"
	}

	jwtCfg := &jwt.Config{
		Email:        strings.TrimSpace(key.ClientEmail),
		PrivateKey:   []byte(key.PrivateKey),
		PrivateKeyID: strings.TrimSpace(key.PrivateKeyID),
		Scopes: []string{
			"https://www.googleapis.com/auth/admin.directory.group.readonly",
		},
		TokenURL: tokenURL,
		Subject:  delegatedAdminEmail,
	}
	token, err := jwtCfg.TokenSource(ctx).Token()
	if err != nil {
		return "", fmt.Errorf("fetch google directory access token: %w", err)
	}
	if strings.TrimSpace(token.AccessToken) == "" {
		return "", fmt.Errorf("google directory token response did not include an access token")
	}
	return strings.TrimSpace(token.AccessToken), nil
}

func doJSON(req *http.Request, target any) error {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s", strings.TrimSpace(string(body)))
	}
	if target == nil {
		return nil
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decode json response: %w", err)
	}
	return nil
}

func requestBearerToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return strings.TrimSpace(authHeader[len("Bearer "):])
	}
	return strings.TrimSpace(r.URL.Query().Get("token"))
}

func googleRedirectURI(r *http.Request) string {
	return requestBaseURL(r) + "/api/auth/google/callback"
}

func oidcRedirectURI(r *http.Request) string {
	return requestBaseURL(r) + "/api/auth/oidc/callback"
}

func requestBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0]); forwarded != "" {
		scheme = forwarded
	}
	host := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Host"), ",")[0])
	if host == "" {
		host = r.Host
	}
	prefix := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Prefix"), ",")[0])
	if prefix != "" && prefix != "/" {
		prefix = "/" + strings.Trim(prefix, "/")
	} else {
		prefix = ""
	}
	return fmt.Sprintf("%s://%s%s", scheme, host, prefix)
}

func oidcScopes(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "openid email profile groups"
	}
	return value
}

func providerLabel(raw, fallback string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback
	}
	return value
}

func requestIsSecure(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0]), "https")
}

func randomStateToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate oauth state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func subtleCompare(a, b string) bool {
	if len(a) != len(b) || a == "" || b == "" {
		return false
	}
	var out byte
	for i := 0; i < len(a); i++ {
		out |= a[i] ^ b[i]
	}
	return out == 0
}
