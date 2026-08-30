package users

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

func (s *Store) Authenticate(ctx context.Context, token string) (*UserWithToken, error) {
	payload, err := s.parseSessionToken(token)
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var user UserWithToken
	switch payload.AuthSource {
	case "local":
		cfgUser, ok := s.users[payload.Username]
		if !ok {
			return nil, sql.ErrNoRows
		}
		roleDef, ok := s.roleDefLocked(cfgUser.Role)
		if !ok {
			return nil, sql.ErrNoRows
		}
		sessionVersion := s.sessionVersions[cfgUser.Username]
		if sessionVersion < 1 {
			sessionVersion = 1
		}
		if sessionVersion != payload.SessionVersion {
			return nil, sql.ErrNoRows
		}
		user = UserWithToken{
			Username:       cfgUser.Username,
			Role:           cfgUser.Role,
			RoleMode:       roleDef.Mode,
			Permissions:    cloneRawMessage(roleDef.Permissions),
			Token:          token,
			AuthSource:     "local",
			SessionVersion: sessionVersion,
		}
	case "google", "oidc", "entra":
		roleDef, ok := s.roleDefLocked(payload.Role)
		if !ok {
			return nil, sql.ErrNoRows
		}
		user = UserWithToken{
			Username:       payload.Username,
			Role:           Role(roleDef.Name),
			RoleMode:       roleDef.Mode,
			Permissions:    cloneRawMessage(roleDef.Permissions),
			Token:          token,
			AuthSource:     payload.AuthSource,
			SessionVersion: 1,
		}
	default:
		return nil, sql.ErrNoRows
	}
	if len(user.Permissions) == 0 {
		user.Permissions = json.RawMessage(`{}`)
	}
	if _, ok := normalizeRoleMode(user.RoleMode); !ok {
		return nil, fmt.Errorf("invalid role mode for user %s", user.Username)
	}
	return &user, nil
}

func (s *Store) VerifyLocalCredentials(ctx context.Context, username, password string) (*UserWithToken, error) {
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	if username == "" || password == "" {
		return nil, sql.ErrNoRows
	}

	s.mu.RLock()
	cfgUser, ok := s.users[username]
	if !ok {
		s.mu.RUnlock()
		return nil, sql.ErrNoRows
	}
	roleDef, ok := s.roleDefLocked(cfgUser.Role)
	if !ok {
		s.mu.RUnlock()
		return nil, sql.ErrNoRows
	}
	passwordMatched, needsUpgrade, err := verifyLocalPassword(cfgUser.PasswordHash, password)
	if err != nil {
		s.mu.RUnlock()
		return nil, err
	}
	if !passwordMatched {
		s.mu.RUnlock()
		return nil, sql.ErrNoRows
	}
	previousPasswordHash := cfgUser.PasswordHash
	sessionVersion := s.sessionVersions[cfgUser.Username]
	if sessionVersion < 1 {
		sessionVersion = 1
	}
	user := &UserWithToken{
		Username:       cfgUser.Username,
		Role:           cfgUser.Role,
		RoleMode:       roleDef.Mode,
		Permissions:    cloneRawMessage(roleDef.Permissions),
		AuthSource:     "local",
		SessionVersion: sessionVersion,
	}
	if len(user.Permissions) == 0 {
		user.Permissions = json.RawMessage(`{}`)
	}
	s.mu.RUnlock()

	if needsUpgrade {
		if passwordHash, hashErr := hashLocalPassword(password); hashErr == nil {
			_ = s.mutateConfigAndPersist(ctx, func() error {
				current, exists := s.users[username]
				if !exists || current.PasswordHash != previousPasswordHash {
					return nil
				}
				current.PasswordHash = passwordHash
				s.users[username] = current
				return nil
			})
		}
	}
	return user, nil
}

func (s *Store) CreateSession(ctx context.Context, username, authSource string) (string, error) {
	username = strings.TrimSpace(username)
	authSource = strings.TrimSpace(strings.ToLower(authSource))
	if username == "" {
		return "", fmt.Errorf("username is required")
	}
	if authSource == "" {
		authSource = "local"
	}
	if authSource != "local" {
		return "", sql.ErrNoRows
	}

	s.mu.RLock()
	cfgUser, ok := s.users[username]
	if !ok {
		s.mu.RUnlock()
		return "", sql.ErrNoRows
	}
	sessionVersion := s.sessionVersions[cfgUser.Username]
	if sessionVersion < 1 {
		sessionVersion = 1
	}
	role := cfgUser.Role
	s.mu.RUnlock()
	return s.newSessionToken(username, authSource, role, sessionVersion)
}

func (s *Store) CreateExternalSession(ctx context.Context, username, authSource string, role Role) (string, error) {
	username = strings.TrimSpace(strings.ToLower(username))
	authSource = strings.TrimSpace(strings.ToLower(authSource))
	role = Role(strings.TrimSpace(strings.ToLower(string(role))))
	if username == "" || authSource == "" {
		return "", fmt.Errorf("external username and auth source are required")
	}
	if authSource != "google" && authSource != "oidc" && authSource != "entra" {
		return "", fmt.Errorf("unsupported external auth source: %s", authSource)
	}
	s.mu.RLock()
	ok := s.roleExistsLocked(string(role))
	s.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("role does not exist: %s", role)
	}
	return s.newSessionToken(username, authSource, role, 1)
}

func (s *Store) List(ctx context.Context) ([]User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]User, 0, len(s.users))
	for _, cfgUser := range s.users {
		out = append(out, User{
			Username:   cfgUser.Username,
			Role:       cfgUser.Role,
			AuthSource: "local",
			CreatedAt:  cfgUser.CreatedAt,
		})
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Username) < strings.ToLower(out[j].Username) })
	return out, nil
}

func (s *Store) Create(ctx context.Context, username, token string, role Role) error {
	username, err := cleanConfigField("username", username, 160, false)
	if err != nil {
		return err
	}
	token = strings.TrimSpace(token)
	role = Role(strings.TrimSpace(strings.ToLower(string(role))))
	if token == "" {
		return fmt.Errorf("username and password are required")
	}
	passwordHash, err := hashLocalPassword(token)
	if err != nil {
		return err
	}

	return s.mutateConfigAndPersist(ctx, func() error {
		if !s.roleExistsLocked(string(role)) {
			return fmt.Errorf("role does not exist: %s", role)
		}
		if s.usernameExistsFoldedLocked(username) {
			return fmt.Errorf("create user: user already exists")
		}
		if s.users == nil {
			s.users = make(map[string]ConfigUser)
		}
		if s.sessionVersions == nil {
			s.sessionVersions = make(map[string]int64)
		}
		s.users[username] = ConfigUser{Username: username, Role: role, PasswordHash: passwordHash, CreatedAt: time.Now().UTC()}
		s.sessionVersions[username] = 1
		return nil
	})
}

func (s *Store) UpdateUserRole(ctx context.Context, username string, role Role) error {
	username = strings.TrimSpace(username)
	role = Role(strings.TrimSpace(strings.ToLower(string(role))))
	if username == "" {
		return fmt.Errorf("username is required")
	}

	return s.mutateConfigAndPersist(ctx, func() error {
		if !s.roleExistsLocked(string(role)) {
			return fmt.Errorf("role does not exist: %s", role)
		}
		cfgUser, ok := s.users[username]
		if !ok {
			return sql.ErrNoRows
		}
		if s.removesLastLocalAdminLocked(username, string(role)) {
			return fmt.Errorf("last local admin role cannot be changed")
		}
		cfgUser.Role = role
		s.users[username] = cfgUser
		return nil
	})
}

func (s *Store) ResetLocalPassword(ctx context.Context, username, password string) error {
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	if username == "" || password == "" {
		return fmt.Errorf("username and password are required")
	}
	passwordHash, err := hashLocalPassword(password)
	if err != nil {
		return err
	}

	return s.mutateConfigAndPersist(ctx, func() error {
		cfgUser, ok := s.users[username]
		if !ok {
			return sql.ErrNoRows
		}
		cfgUser.PasswordHash = passwordHash
		s.users[username] = cfgUser
		if s.sessionVersions == nil {
			s.sessionVersions = make(map[string]int64)
		}
		s.sessionVersions[username] = nextSessionVersion(s.sessionVersions[username])
		return nil
	})
}

func (s *Store) ListRoles(ctx context.Context) ([]RoleDef, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]RoleDef, 0, len(s.roles))
	for _, role := range s.roles {
		role.Permissions = cloneRawMessage(role.Permissions)
		out = append(out, role)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s *Store) CreateRole(ctx context.Context, name, mode string, permissions json.RawMessage) error {
	name = strings.TrimSpace(strings.ToLower(name))
	mode, ok := normalizeRoleMode(mode)
	if name == "" {
		return fmt.Errorf("role name is required")
	}
	if !ok {
		return fmt.Errorf("invalid role mode: %s", mode)
	}
	if name == string(RoleAdmin) && mode != string(RoleAdmin) {
		return fmt.Errorf("admin role mode must stay admin")
	}
	if len(permissions) == 0 {
		permissions = json.RawMessage(`{}`)
	}
	if mode == string(RoleAdmin) {
		permissions = json.RawMessage(`{}`)
	}
	permissions, err := normalizeRolePermissionsJSON(permissions)
	if err != nil {
		return err
	}

	return s.mutateConfigAndPersist(ctx, func() error {
		if s.roleExistsLocked(name) {
			return fmt.Errorf("create role: role already exists")
		}
		if s.roles == nil {
			s.roles = make(map[string]RoleDef)
		}
		s.roles[name] = RoleDef{Name: name, Mode: mode, Permissions: permissions, CreatedAt: time.Now().UTC()}
		return nil
	})
}

func (s *Store) UpdateRole(ctx context.Context, name, mode string, permissions json.RawMessage) error {
	name = strings.TrimSpace(strings.ToLower(name))
	mode, ok := normalizeRoleMode(mode)
	if name == "" {
		return fmt.Errorf("role name is required")
	}
	if !ok {
		return fmt.Errorf("invalid role mode: %s", mode)
	}
	if name == string(RoleAdmin) && mode != string(RoleAdmin) {
		return fmt.Errorf("admin role mode must stay admin")
	}
	if len(permissions) == 0 {
		permissions = json.RawMessage(`{}`)
	}
	if mode == string(RoleAdmin) {
		permissions = json.RawMessage(`{}`)
	}
	permissions, err := normalizeRolePermissionsJSON(permissions)
	if err != nil {
		return err
	}

	return s.mutateConfigAndPersist(ctx, func() error {
		roleDef, exists := s.roles[name]
		if !exists {
			return sql.ErrNoRows
		}
		roleDef.Mode = mode
		roleDef.Permissions = permissions
		s.roles[name] = roleDef
		return nil
	})
}

func (s *Store) DeleteRole(ctx context.Context, name string) error {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return fmt.Errorf("role name is required")
	}
	if name == string(RoleAdmin) {
		return fmt.Errorf("admin role cannot be deleted")
	}

	return s.mutateConfigAndPersist(ctx, func() error {
		if !s.roleExistsLocked(name) {
			return sql.ErrNoRows
		}
		for _, user := range s.users {
			if string(user.Role) == name {
				return fmt.Errorf("role is assigned to users")
			}
		}
		for _, mapping := range s.googleMappings {
			if string(mapping.Role) == name {
				return fmt.Errorf("role is assigned to google group mappings")
			}
		}
		for _, mapping := range s.oidcMappings {
			if string(mapping.Role) == name {
				return fmt.Errorf("role is assigned to OpenID Connect group mappings")
			}
		}
		for _, mapping := range s.entraMappings {
			if string(mapping.Role) == name {
				return fmt.Errorf("role is assigned to Azure Entra ID group mappings")
			}
		}
		delete(s.roles, name)
		return nil
	})
}

func (s *Store) roleExists(ctx context.Context, name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.roleExistsLocked(name)
}

func (s *Store) Delete(ctx context.Context, username string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return fmt.Errorf("username is required")
	}

	return s.mutateConfigAndPersist(ctx, func() error {
		if _, ok := s.users[username]; !ok {
			return sql.ErrNoRows
		}
		if s.isLastLocalAdminLocked(username) {
			return fmt.Errorf("last local admin user cannot be deleted")
		}
		delete(s.users, username)
		delete(s.sessionVersions, username)
		return nil
	})
}

func (s *Store) roleDefLocked(role Role) (RoleDef, bool) {
	roleName := strings.TrimSpace(strings.ToLower(string(role)))
	roleDef, ok := s.roles[roleName]
	return roleDef, ok
}

func (s *Store) roleExistsLocked(name string) bool {
	_, ok := s.roles[strings.TrimSpace(strings.ToLower(name))]
	return ok
}

func (s *Store) usernameExistsFoldedLocked(username string) bool {
	username = strings.ToLower(strings.TrimSpace(username))
	for existing := range s.users {
		if strings.ToLower(existing) == username {
			return true
		}
	}
	return false
}

func (s *Store) isLastLocalAdminLocked(username string) bool {
	cfgUser, ok := s.users[username]
	if !ok {
		return false
	}
	roleDef, ok := s.roleDefLocked(cfgUser.Role)
	if !ok || roleDef.Mode != string(RoleAdmin) {
		return false
	}
	adminCount := 0
	for _, user := range s.users {
		roleDef, ok := s.roleDefLocked(user.Role)
		if ok && roleDef.Mode == string(RoleAdmin) {
			adminCount++
		}
	}
	return adminCount <= 1
}

func (s *Store) removesLastLocalAdminLocked(username, nextRole string) bool {
	if !s.isLastLocalAdminLocked(username) {
		return false
	}
	nextRoleDef, ok := s.roleDefLocked(Role(nextRole))
	return !ok || nextRoleDef.Mode != string(RoleAdmin)
}
