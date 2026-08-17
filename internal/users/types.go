package users

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

type Role string

const (
	RoleAdmin  Role = "admin"
	RoleViewer Role = "viewer"
)

type User struct {
	Username   string    `json:"username"`
	Role       Role      `json:"role"`
	AuthSource string    `json:"auth_source"`
	CreatedAt  time.Time `json:"created_at"`
}

type RoleDef struct {
	Name        string          `json:"name"`
	Mode        string          `json:"mode"`
	Permissions json.RawMessage `json:"permissions,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}

type UserWithToken struct {
	Username       string          `json:"username"`
	Role           Role            `json:"role"`
	RoleMode       string          `json:"role_mode"`
	Permissions    json.RawMessage `json:"permissions,omitempty"`
	Token          string          `json:"token"`
	AuthSource     string          `json:"auth_source"`
	SessionVersion int64           `json:"session_version"`
}

type GoogleConfig struct {
	ClientID            string `json:"client_id"`
	ClientSecret        string `json:"client_secret"`
	HostedDomain        string `json:"hosted_domain"`
	ServiceAccountJSON  string `json:"service_account_json"`
	DelegatedAdminEmail string `json:"delegated_admin_email"`
	UpdatedAt           string `json:"updated_at,omitempty"`
}

type GoogleGroupRole struct {
	GroupEmail string    `json:"group_email"`
	Role       Role      `json:"role"`
	CreatedAt  time.Time `json:"created_at"`
}

type OIDCConfig struct {
	ProviderName string `json:"provider_name"`
	IssuerURL    string `json:"issuer_url"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Scopes       string `json:"scopes"`
	HostedDomain string `json:"hosted_domain"`
	EmailClaim   string `json:"email_claim"`
	GroupsClaim  string `json:"groups_claim"`
	UpdatedAt    string `json:"updated_at,omitempty"`
}

type OIDCGroupRole struct {
	GroupName string    `json:"group_name"`
	Role      Role      `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

type BootstrapStatus struct {
	Initialized bool `json:"initialized"`
}

type Store struct {
	db                *sql.DB
	sessionSigningKey []byte
	mu                sync.RWMutex
	roles             map[string]RoleDef
	users             map[string]ConfigUser
	sessionVersions   map[string]int64
	googleConfig      GoogleConfig
	googleMappings    map[string]GoogleGroupRole
	oidcConfig        OIDCConfig
	oidcMappings      map[string]OIDCGroupRole
	configMutationMu  sync.Mutex
	configSaverMu     sync.RWMutex
	configSaver       func(context.Context, ConfigSnapshot) error
}

func normalizeRoleMode(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(RoleAdmin):
		return string(RoleAdmin), true
	case string(RoleViewer):
		return string(RoleViewer), true
	default:
		return "", false
	}
}

func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate random token: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
