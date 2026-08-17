package users

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
	"unicode"

	"sigs.k8s.io/yaml"
)

const (
	ConfigSnapshotSchemaVersion = 1
	defaultOIDCScopes           = "openid email profile groups"
	defaultOIDCEmailClaim       = "email"
	defaultOIDCGroupsClaim      = "groups"
)

type ConfigSnapshot struct {
	SchemaVersion int              `json:"schema_version"`
	ExportedAt    string           `json:"exported_at,omitempty"`
	Initialized   bool             `json:"initialized"`
	Roles         []RoleDef        `json:"roles"`
	Users         []ConfigUser     `json:"users"`
	Google        ConfigGoogleAuth `json:"google"`
	OIDC          ConfigOIDCAuth   `json:"oidc"`
}

type ConfigUser struct {
	Username     string    `json:"username"`
	Role         Role      `json:"role"`
	PasswordHash string    `json:"password_hash"`
	CreatedAt    time.Time `json:"created_at,omitempty"`
}

type configSnapshotYAML struct {
	SchemaVersion int                  `json:"schema_version"`
	ExportedAt    string               `json:"exported_at,omitempty"`
	Initialized   bool                 `json:"initialized"`
	Roles         []configRoleYAML     `json:"roles"`
	Users         []configUserYAML     `json:"users"`
	Google        configGoogleAuthYAML `json:"google"`
	OIDC          configOIDCAuthYAML   `json:"oidc"`
}

type configRoleYAML struct {
	Name        string                 `json:"name"`
	Mode        string                 `json:"mode"`
	Permissions *configPermissionsYAML `json:"permissions,omitempty"`
}

type configUserYAML struct {
	Username     string `json:"username"`
	Role         Role   `json:"role"`
	PasswordHash string `json:"password_hash"`
}

type configPermissionsYAML struct {
	Namespaces []string          `json:"namespaces,omitempty"`
	Resources  map[string]string `json:"resources,omitempty"`
}

type configSnapshotOutputYAML struct {
	SchemaVersion int                    `json:"schema_version"`
	ExportedAt    string                 `json:"exported_at,omitempty"`
	Initialized   bool                   `json:"initialized"`
	Roles         []configRoleOutputYAML `json:"roles"`
	Users         []configUserYAML       `json:"users"`
	Google        configGoogleAuthYAML   `json:"google"`
	OIDC          configOIDCAuthYAML     `json:"oidc"`
}

type configRoleOutputYAML struct {
	Name        string                 `json:"name"`
	Mode        string                 `json:"mode"`
	Permissions *configPermissionsYAML `json:"permissions,omitempty"`
}

type configGoogleAuthYAML struct {
	Config   GoogleConfig              `json:"config"`
	Mappings []configGoogleMappingYAML `json:"mappings"`
}

type configGoogleMappingYAML struct {
	GroupEmail string `json:"group_email"`
	Role       Role   `json:"role"`
}

type configOIDCAuthYAML struct {
	Config   OIDCConfig              `json:"config"`
	Mappings []configOIDCMappingYAML `json:"mappings"`
}

type configOIDCMappingYAML struct {
	GroupName string `json:"group_name"`
	Role      Role   `json:"role"`
}

type ConfigGoogleAuth struct {
	Config   GoogleConfig      `json:"config"`
	Mappings []GoogleGroupRole `json:"mappings"`
}

type ConfigOIDCAuth struct {
	Config   OIDCConfig      `json:"config"`
	Mappings []OIDCGroupRole `json:"mappings"`
}

type ConfigImportError struct {
	Stage string
	Err   error
}

func (e *ConfigImportError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("config import failed at %s: %v", e.Stage, e.Err)
}

func (e *ConfigImportError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func ConfigImportStage(err error) string {
	var importErr *ConfigImportError
	if errors.As(err, &importErr) {
		return importErr.Stage
	}
	return ""
}

func importStageError(stage string, err error) error {
	if err == nil {
		return nil
	}
	return &ConfigImportError{Stage: stage, Err: err}
}

func DecodeConfigSnapshot(r io.Reader) (ConfigSnapshot, error) {
	data, err := io.ReadAll(io.LimitReader(r, 2*1024*1024+1))
	if err != nil {
		return ConfigSnapshot{}, importStageError("decode", err)
	}
	if len(data) > 2*1024*1024 {
		return ConfigSnapshot{}, importStageError("decode", fmt.Errorf("config snapshot is too large"))
	}
	var raw configSnapshotYAML
	if err := yaml.UnmarshalStrict(data, &raw); err != nil {
		return ConfigSnapshot{}, importStageError("decode", err)
	}
	snapshot, err := configSnapshotFromYAML(raw)
	if err != nil {
		return ConfigSnapshot{}, importStageError("decode", err)
	}
	return snapshot, nil
}

func DecodeConfigSnapshotBytes(data []byte) (ConfigSnapshot, error) {
	return DecodeConfigSnapshot(bytes.NewReader(data))
}

func EncodeConfigSnapshot(snapshot ConfigSnapshot) ([]byte, error) {
	if snapshot.SchemaVersion == 0 {
		snapshot.SchemaVersion = ConfigSnapshotSchemaVersion
	}
	if strings.TrimSpace(snapshot.ExportedAt) == "" {
		snapshot.ExportedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	raw, err := configSnapshotToYAML(snapshot)
	if err != nil {
		return nil, err
	}
	return yaml.Marshal(raw)
}

func configSnapshotFromYAML(raw configSnapshotYAML) (ConfigSnapshot, error) {
	roles := make([]RoleDef, 0, len(raw.Roles))
	for _, role := range raw.Roles {
		permissions, err := expandedPermissionsJSON(role.Permissions)
		if err != nil {
			return ConfigSnapshot{}, err
		}
		roles = append(roles, RoleDef{
			Name:        role.Name,
			Mode:        role.Mode,
			Permissions: permissions,
		})
	}
	users := make([]ConfigUser, 0, len(raw.Users))
	for _, user := range raw.Users {
		users = append(users, ConfigUser{
			Username:     user.Username,
			Role:         user.Role,
			PasswordHash: user.PasswordHash,
		})
	}
	googleMappings := make([]GoogleGroupRole, 0, len(raw.Google.Mappings))
	for _, item := range raw.Google.Mappings {
		googleMappings = append(googleMappings, GoogleGroupRole{
			GroupEmail: item.GroupEmail,
			Role:       item.Role,
		})
	}
	oidcMappings := make([]OIDCGroupRole, 0, len(raw.OIDC.Mappings))
	for _, item := range raw.OIDC.Mappings {
		oidcMappings = append(oidcMappings, OIDCGroupRole{
			GroupName: item.GroupName,
			Role:      item.Role,
		})
	}
	return ConfigSnapshot{
		SchemaVersion: raw.SchemaVersion,
		ExportedAt:    raw.ExportedAt,
		Initialized:   raw.Initialized,
		Roles:         roles,
		Users:         users,
		Google:        ConfigGoogleAuth{Config: raw.Google.Config, Mappings: googleMappings},
		OIDC:          ConfigOIDCAuth{Config: raw.OIDC.Config, Mappings: oidcMappings},
	}, nil
}

func configSnapshotToYAML(snapshot ConfigSnapshot) (configSnapshotOutputYAML, error) {
	roles := make([]configRoleOutputYAML, 0, len(snapshot.Roles))
	for _, role := range snapshot.Roles {
		compact, err := compactPermissionsYAML(role.Permissions)
		if err != nil {
			return configSnapshotOutputYAML{}, err
		}
		if role.Mode == string(RoleAdmin) {
			compact = nil
		}
		roles = append(roles, configRoleOutputYAML{
			Name:        role.Name,
			Mode:        role.Mode,
			Permissions: compact,
		})
	}
	users := make([]configUserYAML, 0, len(snapshot.Users))
	for _, user := range snapshot.Users {
		users = append(users, configUserYAML{
			Username:     user.Username,
			Role:         user.Role,
			PasswordHash: user.PasswordHash,
		})
	}
	googleMappings := make([]configGoogleMappingYAML, 0, len(snapshot.Google.Mappings))
	for _, item := range snapshot.Google.Mappings {
		googleMappings = append(googleMappings, configGoogleMappingYAML{GroupEmail: item.GroupEmail, Role: item.Role})
	}
	oidcMappings := make([]configOIDCMappingYAML, 0, len(snapshot.OIDC.Mappings))
	for _, item := range snapshot.OIDC.Mappings {
		oidcMappings = append(oidcMappings, configOIDCMappingYAML{GroupName: item.GroupName, Role: item.Role})
	}
	return configSnapshotOutputYAML{
		SchemaVersion: snapshot.SchemaVersion,
		ExportedAt:    snapshot.ExportedAt,
		Initialized:   snapshot.Initialized,
		Roles:         roles,
		Users:         users,
		Google:        configGoogleAuthYAML{Config: snapshot.Google.Config, Mappings: googleMappings},
		OIDC:          configOIDCAuthYAML{Config: snapshot.OIDC.Config, Mappings: oidcMappings},
	}, nil
}

func (s *Store) SetConfigSaver(fn func(context.Context, ConfigSnapshot) error) {
	s.configSaverMu.Lock()
	defer s.configSaverMu.Unlock()
	s.configSaver = fn
}

func (s *Store) PersistConfig(ctx context.Context) error {
	if s == nil {
		return nil
	}
	s.configMutationMu.Lock()
	defer s.configMutationMu.Unlock()

	s.mu.RLock()
	snapshot := s.snapshotLocked()
	snapshot.ExportedAt = time.Now().UTC().Format(time.RFC3339Nano)
	s.mu.RUnlock()
	return s.saveConfigSnapshot(ctx, snapshot)
}

func (s *Store) SaveConfigSnapshot(ctx context.Context, snapshot ConfigSnapshot) error {
	if s == nil {
		return nil
	}
	s.configMutationMu.Lock()
	defer s.configMutationMu.Unlock()
	return s.saveConfigSnapshot(ctx, snapshot)
}

func (s *Store) saveConfigSnapshot(ctx context.Context, snapshot ConfigSnapshot) error {
	s.configSaverMu.RLock()
	save := s.configSaver
	s.configSaverMu.RUnlock()
	if save == nil {
		return nil
	}
	return save(ctx, snapshot)
}

func (s *Store) persistConfig(ctx context.Context) error {
	return s.PersistConfig(ctx)
}

func (s *Store) ExportConfig(ctx context.Context) (ConfigSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snapshot := s.snapshotLocked()
	snapshot.ExportedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return snapshot, nil
}

func (s *Store) ImportConfigSnapshot(ctx context.Context, snapshot ConfigSnapshot) error {
	normalized, err := NormalizeConfigSnapshot(snapshot)
	if err != nil {
		return err
	}
	return s.replaceConfig(ctx, normalized)
}

// ReplaceConfigSnapshot persists a validated snapshot and only exposes it to
// readers after durable storage accepts it. It is intended for runtime imports;
// startup restoration should continue to use ImportConfigSnapshot.
func (s *Store) ReplaceConfigSnapshot(ctx context.Context, snapshot ConfigSnapshot) error {
	normalized, err := NormalizeConfigSnapshot(snapshot)
	if err != nil {
		return err
	}
	return s.mutateConfigAndPersist(ctx, func() error {
		s.applyConfigSnapshotLocked(normalized)
		return nil
	})
}

func (s *Store) ResetToEmptyConfig(ctx context.Context) error {
	return s.replaceConfig(ctx, defaultConfigSnapshot(time.Now().UTC()))
}

// mutateConfigAndPersist serializes configuration changes, keeps readers from
// observing uncommitted state and restores both configuration and session
// versions if the external persistence callback fails.
func (s *Store) mutateConfigAndPersist(ctx context.Context, mutate func() error) error {
	if s == nil {
		return nil
	}
	s.configMutationMu.Lock()
	defer s.configMutationMu.Unlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	previous := s.snapshotLocked()
	previousSessionVersions := cloneSessionVersions(s.sessionVersions)
	if err := mutate(); err != nil {
		return err
	}
	next := s.snapshotLocked()
	if err := s.saveConfigSnapshot(ctx, next); err != nil {
		s.applyConfigSnapshotLocked(previous)
		s.sessionVersions = previousSessionVersions
		return err
	}
	return nil
}

func cloneSessionVersions(input map[string]int64) map[string]int64 {
	if input == nil {
		return nil
	}
	out := make(map[string]int64, len(input))
	for username, version := range input {
		out[username] = version
	}
	return out
}

func NormalizeConfigSnapshot(snapshot ConfigSnapshot) (ConfigSnapshot, error) {
	if snapshot.SchemaVersion == 0 {
		snapshot.SchemaVersion = ConfigSnapshotSchemaVersion
	}
	if snapshot.SchemaVersion != ConfigSnapshotSchemaVersion {
		return ConfigSnapshot{}, importStageError("schema", fmt.Errorf("unsupported schema_version %d", snapshot.SchemaVersion))
	}
	now := time.Now().UTC()
	roles, roleDefs, err := normalizeConfigRoles(snapshot.Roles, now)
	if err != nil {
		return ConfigSnapshot{}, err
	}
	users, err := normalizeConfigUsers(snapshot.Users, roleDefs, snapshot.Initialized, now)
	if err != nil {
		return ConfigSnapshot{}, err
	}
	google, err := normalizeConfigGoogle(snapshot.Google, roleDefs, now)
	if err != nil {
		return ConfigSnapshot{}, err
	}
	oidc, err := normalizeConfigOIDC(snapshot.OIDC, roleDefs, now)
	if err != nil {
		return ConfigSnapshot{}, err
	}

	return ConfigSnapshot{
		SchemaVersion: ConfigSnapshotSchemaVersion,
		ExportedAt:    strings.TrimSpace(snapshot.ExportedAt),
		Initialized:   configHasLocalAdmin(users, roleDefs),
		Roles:         roles,
		Users:         users,
		Google:        google,
		OIDC:          oidc,
	}, nil
}

func normalizeConfigRoles(input []RoleDef, now time.Time) ([]RoleDef, map[string]RoleDef, error) {
	seen := make(map[string]RoleDef, len(input)+2)
	for _, role := range input {
		name, err := cleanConfigField("role name", strings.ToLower(role.Name), 80, false)
		if err != nil {
			return nil, nil, importStageError("roles", err)
		}
		mode, ok := normalizeRoleMode(role.Mode)
		if !ok {
			return nil, nil, importStageError("roles", fmt.Errorf("invalid role mode for %s", name))
		}
		if name == string(RoleAdmin) && mode != string(RoleAdmin) {
			return nil, nil, importStageError("roles", fmt.Errorf("admin role mode must stay admin"))
		}
		perms := role.Permissions
		if len(perms) == 0 {
			perms = json.RawMessage(`{}`)
		}
		if mode == string(RoleAdmin) {
			perms = json.RawMessage(`{}`)
		}
		normalizedPerms, err := normalizeRolePermissionsJSON(perms)
		if err != nil {
			return nil, nil, importStageError("roles", fmt.Errorf("permissions for role %s must be valid json", name))
		}
		createdAt := role.CreatedAt
		if createdAt.IsZero() {
			createdAt = now
		}
		if _, ok := seen[name]; ok {
			return nil, nil, importStageError("roles", fmt.Errorf("duplicate role %s", name))
		}
		seen[name] = RoleDef{Name: name, Mode: mode, Permissions: normalizedPerms, CreatedAt: createdAt.UTC()}
	}
	if _, ok := seen[string(RoleAdmin)]; !ok {
		seen[string(RoleAdmin)] = RoleDef{Name: string(RoleAdmin), Mode: string(RoleAdmin), Permissions: json.RawMessage(`{}`), CreatedAt: now}
	}

	out := make([]RoleDef, 0, len(seen))
	roleDefs := make(map[string]RoleDef, len(seen))
	for name, role := range seen {
		out = append(out, role)
		roleDefs[name] = role
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, roleDefs, nil
}

func normalizeConfigUsers(input []ConfigUser, roleDefs map[string]RoleDef, initialized bool, now time.Time) ([]ConfigUser, error) {
	out := make([]ConfigUser, 0, len(input))
	seen := make(map[string]struct{}, len(input))
	hasAdmin := false
	for _, user := range input {
		username, err := cleanConfigField("username", user.Username, 160, false)
		if err != nil {
			return nil, importStageError("users", err)
		}
		if _, ok := seen[strings.ToLower(username)]; ok {
			return nil, importStageError("users", fmt.Errorf("duplicate user %s", username))
		}
		seen[strings.ToLower(username)] = struct{}{}
		role := Role(strings.TrimSpace(strings.ToLower(string(user.Role))))
		roleDef, ok := roleDefs[string(role)]
		if !ok {
			return nil, importStageError("users", fmt.Errorf("user %s references missing role %s", username, role))
		}
		if !isLocalPasswordHash(user.PasswordHash) {
			return nil, importStageError("users", fmt.Errorf("user %s must contain a bdk1 password_hash, not a raw password", username))
		}
		createdAt := user.CreatedAt
		if createdAt.IsZero() {
			createdAt = now
		}
		if roleDef.Mode == string(RoleAdmin) {
			hasAdmin = true
		}
		out = append(out, ConfigUser{Username: username, Role: role, PasswordHash: strings.TrimSpace(user.PasswordHash), CreatedAt: createdAt.UTC()})
	}
	if initialized && !hasAdmin {
		return nil, importStageError("users", fmt.Errorf("initialized config must include local admin user with admin role"))
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Username) < strings.ToLower(out[j].Username) })
	return out, nil
}

func configHasLocalAdmin(users []ConfigUser, roleDefs map[string]RoleDef) bool {
	for _, user := range users {
		roleDef, ok := roleDefs[string(user.Role)]
		if ok && roleDef.Mode == string(RoleAdmin) {
			return true
		}
	}
	return false
}

func normalizeConfigGoogle(input ConfigGoogleAuth, roleDefs map[string]RoleDef, now time.Time) (ConfigGoogleAuth, error) {
	cfg := GoogleConfig{
		ClientID:            strings.TrimSpace(input.Config.ClientID),
		ClientSecret:        strings.TrimSpace(input.Config.ClientSecret),
		HostedDomain:        strings.TrimSpace(strings.ToLower(input.Config.HostedDomain)),
		ServiceAccountJSON:  strings.TrimSpace(input.Config.ServiceAccountJSON),
		DelegatedAdminEmail: strings.TrimSpace(strings.ToLower(input.Config.DelegatedAdminEmail)),
		UpdatedAt:           strings.TrimSpace(input.Config.UpdatedAt),
	}
	if err := validateConfigTextFields("google config", map[string]string{
		"client_id":             cfg.ClientID,
		"client_secret":         cfg.ClientSecret,
		"hosted_domain":         cfg.HostedDomain,
		"delegated_admin_email": cfg.DelegatedAdminEmail,
	}, 4096); err != nil {
		return ConfigGoogleAuth{}, importStageError("google config", err)
	}
	if strings.ContainsRune(cfg.ServiceAccountJSON, '\x00') {
		return ConfigGoogleAuth{}, importStageError("google config", fmt.Errorf("service_account_json contains invalid NUL byte"))
	}
	if len(cfg.ServiceAccountJSON) > 128*1024 {
		return ConfigGoogleAuth{}, importStageError("google config", fmt.Errorf("service_account_json is too large"))
	}

	mappings := make([]GoogleGroupRole, 0, len(input.Mappings))
	seen := make(map[string]struct{}, len(input.Mappings))
	for _, item := range input.Mappings {
		groupEmail, err := cleanConfigField("google group email", strings.ToLower(item.GroupEmail), 320, false)
		if err != nil {
			return ConfigGoogleAuth{}, importStageError("google mappings", err)
		}
		if _, ok := seen[groupEmail]; ok {
			return ConfigGoogleAuth{}, importStageError("google mappings", fmt.Errorf("duplicate google group %s", groupEmail))
		}
		seen[groupEmail] = struct{}{}
		role := Role(strings.TrimSpace(strings.ToLower(string(item.Role))))
		if _, ok := roleDefs[string(role)]; !ok {
			return ConfigGoogleAuth{}, importStageError("google mappings", fmt.Errorf("google group %s references missing role %s", groupEmail, role))
		}
		createdAt := item.CreatedAt
		if createdAt.IsZero() {
			createdAt = now
		}
		mappings = append(mappings, GoogleGroupRole{GroupEmail: groupEmail, Role: role, CreatedAt: createdAt.UTC()})
	}
	sort.Slice(mappings, func(i, j int) bool { return mappings[i].GroupEmail < mappings[j].GroupEmail })
	return ConfigGoogleAuth{Config: cfg, Mappings: mappings}, nil
}

func normalizeConfigOIDC(input ConfigOIDCAuth, roleDefs map[string]RoleDef, now time.Time) (ConfigOIDCAuth, error) {
	providerName := strings.TrimSpace(input.Config.ProviderName)
	if providerName == "" {
		providerName = "OpenID Connect"
	}
	scopes := strings.TrimSpace(input.Config.Scopes)
	if scopes == "" {
		scopes = defaultOIDCScopes
	}
	emailClaim := strings.TrimSpace(input.Config.EmailClaim)
	if emailClaim == "" {
		emailClaim = defaultOIDCEmailClaim
	}
	groupsClaim := strings.TrimSpace(input.Config.GroupsClaim)
	if groupsClaim == "" {
		groupsClaim = defaultOIDCGroupsClaim
	}
	cfg := OIDCConfig{
		ProviderName: providerName,
		IssuerURL:    strings.TrimSpace(input.Config.IssuerURL),
		ClientID:     strings.TrimSpace(input.Config.ClientID),
		ClientSecret: strings.TrimSpace(input.Config.ClientSecret),
		Scopes:       scopes,
		HostedDomain: strings.TrimSpace(strings.ToLower(input.Config.HostedDomain)),
		EmailClaim:   emailClaim,
		GroupsClaim:  groupsClaim,
		UpdatedAt:    strings.TrimSpace(input.Config.UpdatedAt),
	}
	if err := validateConfigTextFields("oidc config", map[string]string{
		"provider_name": cfg.ProviderName,
		"issuer_url":    cfg.IssuerURL,
		"client_id":     cfg.ClientID,
		"client_secret": cfg.ClientSecret,
		"scopes":        cfg.Scopes,
		"hosted_domain": cfg.HostedDomain,
		"email_claim":   cfg.EmailClaim,
		"groups_claim":  cfg.GroupsClaim,
	}, 4096); err != nil {
		return ConfigOIDCAuth{}, importStageError("oidc config", err)
	}

	mappings := make([]OIDCGroupRole, 0, len(input.Mappings))
	seen := make(map[string]struct{}, len(input.Mappings))
	for _, item := range input.Mappings {
		groupName, err := cleanConfigField("oidc group", item.GroupName, 512, false)
		if err != nil {
			return ConfigOIDCAuth{}, importStageError("oidc mappings", err)
		}
		groupKey := strings.ToLower(groupName)
		if _, ok := seen[groupKey]; ok {
			return ConfigOIDCAuth{}, importStageError("oidc mappings", fmt.Errorf("duplicate oidc group %s", groupName))
		}
		seen[groupKey] = struct{}{}
		role := Role(strings.TrimSpace(strings.ToLower(string(item.Role))))
		if _, ok := roleDefs[string(role)]; !ok {
			return ConfigOIDCAuth{}, importStageError("oidc mappings", fmt.Errorf("oidc group %s references missing role %s", groupName, role))
		}
		createdAt := item.CreatedAt
		if createdAt.IsZero() {
			createdAt = now
		}
		mappings = append(mappings, OIDCGroupRole{GroupName: groupName, Role: role, CreatedAt: createdAt.UTC()})
	}
	sort.Slice(mappings, func(i, j int) bool {
		return strings.ToLower(mappings[i].GroupName) < strings.ToLower(mappings[j].GroupName)
	})
	return ConfigOIDCAuth{Config: cfg, Mappings: mappings}, nil
}

type normalizedRolePermissions struct {
	Namespaces []string
	Resources  map[string]rolePermissionFlags
}

type rolePermissionFlags struct {
	View   bool `json:"view"`
	Edit   bool `json:"edit"`
	Delete bool `json:"delete"`
}

func normalizeRolePermissionsJSON(raw json.RawMessage) (json.RawMessage, error) {
	permissions, err := parseRolePermissionsJSON(raw)
	if err != nil {
		return nil, err
	}
	return expandedPermissionsJSONFromNormalized(permissions)
}

func expandedPermissionsJSON(compact *configPermissionsYAML) (json.RawMessage, error) {
	if compact == nil {
		return json.RawMessage(`{}`), nil
	}
	namespaces, err := normalizePermissionNamespaces(compact.Namespaces)
	if err != nil {
		return nil, err
	}
	permissions := normalizedRolePermissions{
		Namespaces: namespaces,
		Resources:  make(map[string]rolePermissionFlags, len(compact.Resources)),
	}
	for resource, level := range compact.Resources {
		resource, err := cleanConfigField("permission resource", strings.ToLower(resource), 80, false)
		if err != nil {
			return nil, err
		}
		flags, ok := permissionLevelToFlags(level)
		if !ok {
			return nil, fmt.Errorf("invalid permission level %q for %s", level, resource)
		}
		if hasAnyPermission(flags) {
			permissions.Resources[resource] = flags
		}
	}
	return expandedPermissionsJSONFromNormalized(permissions)
}

func expandedPermissionsJSONFromNormalized(permissions normalizedRolePermissions) (json.RawMessage, error) {
	namespaces, err := normalizePermissionNamespaces(permissions.Namespaces)
	if err != nil {
		return nil, err
	}
	resources := make(map[string]rolePermissionFlags)
	for resource, flags := range permissions.Resources {
		if hasAnyPermission(flags) {
			resources[resource] = flags
		}
	}
	if len(resources) == 0 {
		return json.RawMessage(`{}`), nil
	}
	root := map[string]any{"resources": resources}
	if len(namespaces) > 0 {
		root["namespaces"] = namespaces
	}
	data, err := json.Marshal(root)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

func compactPermissionsYAML(raw json.RawMessage) (*configPermissionsYAML, error) {
	permissions, err := parseRolePermissionsJSON(raw)
	if err != nil {
		return nil, err
	}
	out := &configPermissionsYAML{
		Resources: make(map[string]string),
	}
	out.Namespaces, err = normalizePermissionNamespaces(permissions.Namespaces)
	if err != nil {
		return nil, err
	}
	resourceNames := make([]string, 0, len(permissions.Resources))
	for resource := range permissions.Resources {
		resourceNames = append(resourceNames, resource)
	}
	sort.Strings(resourceNames)
	for _, resource := range resourceNames {
		level := permissionFlagsToLevel(permissions.Resources[resource])
		if level != "" {
			out.Resources[resource] = level
		}
	}
	if len(out.Resources) == 0 {
		return nil, nil
	}
	return out, nil
}

func parseRolePermissionsJSON(raw json.RawMessage) (normalizedRolePermissions, error) {
	out := normalizedRolePermissions{Resources: map[string]rolePermissionFlags{}}
	if len(bytes.TrimSpace(raw)) == 0 || string(bytes.TrimSpace(raw)) == "null" {
		return out, nil
	}
	if !json.Valid(raw) {
		return out, fmt.Errorf("permissions must be valid json")
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil {
		return out, fmt.Errorf("permissions must be a json object")
	}
	for key, value := range root {
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "namespaces":
			var namespaces []string
			if err := json.Unmarshal(value, &namespaces); err != nil {
				return out, fmt.Errorf("permissions namespaces must be a string array")
			}
			normalized, err := normalizePermissionNamespaces(namespaces)
			if err != nil {
				return out, err
			}
			out.Namespaces = normalized
		case "resources":
			resources, err := parsePermissionResources(value)
			if err != nil {
				return out, err
			}
			out.Resources = resources
		default:
			return out, fmt.Errorf("unknown permissions field %q", key)
		}
	}
	return out, nil
}

func parsePermissionResources(raw json.RawMessage) (map[string]rolePermissionFlags, error) {
	var resources map[string]json.RawMessage
	if err := json.Unmarshal(raw, &resources); err != nil {
		return nil, fmt.Errorf("permissions resources must be an object")
	}
	out := make(map[string]rolePermissionFlags, len(resources))
	for resource, rawValue := range resources {
		name, err := cleanConfigField("permission resource", strings.ToLower(resource), 80, false)
		if err != nil {
			return nil, err
		}
		flags, err := parsePermissionResourceValue(name, rawValue)
		if err != nil {
			return nil, err
		}
		if hasAnyPermission(flags) {
			out[name] = flags
		}
	}
	return out, nil
}

func parsePermissionResourceValue(resource string, raw json.RawMessage) (rolePermissionFlags, error) {
	var level string
	if err := json.Unmarshal(raw, &level); err == nil {
		flags, ok := permissionLevelToFlags(level)
		if !ok {
			return rolePermissionFlags{}, fmt.Errorf("invalid permission level %q for %s", level, resource)
		}
		return flags, nil
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return rolePermissionFlags{}, fmt.Errorf("permissions for %s must be a level string or object", resource)
	}
	flags := rolePermissionFlags{}
	for key, value := range fields {
		var enabled bool
		if err := json.Unmarshal(value, &enabled); err != nil {
			return rolePermissionFlags{}, fmt.Errorf("permission %s.%s must be boolean", resource, key)
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "view":
			flags.View = enabled
		case "edit":
			flags.Edit = enabled
		case "delete":
			flags.Delete = enabled
		default:
			return rolePermissionFlags{}, fmt.Errorf("unknown permission action %s.%s", resource, key)
		}
	}
	if flags.Delete {
		flags.Edit = true
		flags.View = true
	}
	if flags.Edit {
		flags.View = true
	}
	return flags, nil
}

func normalizePermissionNamespaces(input []string) ([]string, error) {
	seen := make(map[string]struct{}, len(input))
	out := make([]string, 0, len(input))
	for _, namespace := range input {
		namespace = strings.TrimSpace(namespace)
		if namespace == "" {
			continue
		}
		if _, err := cleanConfigField("permission namespace", namespace, 253, false); err != nil {
			return nil, err
		}
		if _, ok := seen[namespace]; ok {
			continue
		}
		seen[namespace] = struct{}{}
		out = append(out, namespace)
	}
	sort.Strings(out)
	return out, nil
}

func permissionLevelToFlags(level string) (rolePermissionFlags, bool) {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "", "none":
		return rolePermissionFlags{}, true
	case "view":
		return rolePermissionFlags{View: true}, true
	case "edit":
		return rolePermissionFlags{View: true, Edit: true}, true
	case "full", "delete":
		return rolePermissionFlags{View: true, Edit: true, Delete: true}, true
	default:
		return rolePermissionFlags{}, false
	}
}

func permissionFlagsToLevel(flags rolePermissionFlags) string {
	if flags.Delete {
		return "full"
	}
	if flags.Edit {
		return "edit"
	}
	if flags.View {
		return "view"
	}
	return ""
}

func hasAnyPermission(flags rolePermissionFlags) bool {
	return flags.View || flags.Edit || flags.Delete
}

func (s *Store) replaceConfig(ctx context.Context, snapshot ConfigSnapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.applyConfigSnapshotLocked(snapshot)
	return nil
}

func defaultConfigSnapshot(now time.Time) ConfigSnapshot {
	return ConfigSnapshot{
		SchemaVersion: ConfigSnapshotSchemaVersion,
		Initialized:   false,
		Roles: []RoleDef{
			{Name: string(RoleAdmin), Mode: string(RoleAdmin), Permissions: json.RawMessage(`{}`), CreatedAt: now},
		},
		Google: ConfigGoogleAuth{Config: GoogleConfig{}},
		OIDC: ConfigOIDCAuth{Config: OIDCConfig{
			ProviderName: "OpenID Connect",
			Scopes:       defaultOIDCScopes,
			EmailClaim:   defaultOIDCEmailClaim,
			GroupsClaim:  defaultOIDCGroupsClaim,
		}},
	}
}

func (s *Store) applyConfigSnapshotLocked(snapshot ConfigSnapshot) {
	roles := make(map[string]RoleDef, len(snapshot.Roles))
	for _, role := range snapshot.Roles {
		role.Permissions = cloneRawMessage(role.Permissions)
		roles[role.Name] = role
	}
	users := make(map[string]ConfigUser, len(snapshot.Users))
	nextVersions := make(map[string]int64, len(snapshot.Users))
	for _, user := range snapshot.Users {
		users[user.Username] = user
		version := s.sessionVersions[user.Username]
		if version < 1 {
			version = 1
		} else {
			version++
		}
		nextVersions[user.Username] = version
	}
	googleMappings := make(map[string]GoogleGroupRole, len(snapshot.Google.Mappings))
	for _, item := range snapshot.Google.Mappings {
		googleMappings[strings.ToLower(item.GroupEmail)] = item
	}
	oidcMappings := make(map[string]OIDCGroupRole, len(snapshot.OIDC.Mappings))
	for _, item := range snapshot.OIDC.Mappings {
		oidcMappings[strings.ToLower(strings.TrimSpace(item.GroupName))] = item
	}
	s.roles = roles
	s.users = users
	s.sessionVersions = nextVersions
	s.googleConfig = snapshot.Google.Config
	s.googleMappings = googleMappings
	s.oidcConfig = snapshot.OIDC.Config
	s.oidcMappings = oidcMappings
}

func (s *Store) snapshotLocked() ConfigSnapshot {
	roles := make([]RoleDef, 0, len(s.roles))
	for _, role := range s.roles {
		role.Permissions = cloneRawMessage(role.Permissions)
		roles = append(roles, role)
	}
	sort.Slice(roles, func(i, j int) bool { return roles[i].Name < roles[j].Name })

	users := make([]ConfigUser, 0, len(s.users))
	for _, user := range s.users {
		users = append(users, user)
	}
	sort.Slice(users, func(i, j int) bool { return strings.ToLower(users[i].Username) < strings.ToLower(users[j].Username) })

	googleMappings := make([]GoogleGroupRole, 0, len(s.googleMappings))
	for _, item := range s.googleMappings {
		googleMappings = append(googleMappings, item)
	}
	sort.Slice(googleMappings, func(i, j int) bool { return googleMappings[i].GroupEmail < googleMappings[j].GroupEmail })

	oidcMappings := make([]OIDCGroupRole, 0, len(s.oidcMappings))
	for _, item := range s.oidcMappings {
		oidcMappings = append(oidcMappings, item)
	}
	sort.Slice(oidcMappings, func(i, j int) bool {
		return strings.ToLower(oidcMappings[i].GroupName) < strings.ToLower(oidcMappings[j].GroupName)
	})

	return ConfigSnapshot{
		SchemaVersion: ConfigSnapshotSchemaVersion,
		Initialized:   s.hasLocalAdminLocked(),
		Roles:         roles,
		Users:         users,
		Google:        ConfigGoogleAuth{Config: s.googleConfig, Mappings: googleMappings},
		OIDC:          ConfigOIDCAuth{Config: s.oidcConfig, Mappings: oidcMappings},
	}
}

func cloneRawMessage(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	out := make([]byte, len(raw))
	copy(out, raw)
	return json.RawMessage(out)
}

func cleanConfigField(name, value string, maxLen int, allowEmpty bool) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" && !allowEmpty {
		return "", fmt.Errorf("%s is required", name)
	}
	if len(value) > maxLen {
		return "", fmt.Errorf("%s is too long", name)
	}
	for _, r := range value {
		if r == '\x00' || r == '\r' || r == '\n' || (unicode.IsControl(r) && r != '\t') {
			return "", fmt.Errorf("%s contains control characters", name)
		}
	}
	return value, nil
}

func validateConfigTextFields(prefix string, values map[string]string, maxLen int) error {
	for name, value := range values {
		if _, err := cleanConfigField(prefix+" "+name, value, maxLen, true); err != nil {
			return err
		}
	}
	return nil
}

func isLocalPasswordHash(value string) bool {
	parts := strings.Split(strings.TrimSpace(value), "$")
	if len(parts) != 4 || parts[0] != localPasswordHashPrefix {
		return false
	}
	return parts[1] != "" && parts[2] != "" && parts[3] != ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
