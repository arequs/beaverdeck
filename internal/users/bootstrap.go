package users

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func (s *Store) EnsureDefaults(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureDefaultsLocked(time.Now().UTC())
	return nil
}

func (s *Store) ensureDefaultsLocked(now time.Time) {
	if s.roles == nil {
		s.roles = make(map[string]RoleDef)
	}
	if _, ok := s.roles[string(RoleAdmin)]; !ok {
		s.roles[string(RoleAdmin)] = RoleDef{Name: string(RoleAdmin), Mode: string(RoleAdmin), Permissions: json.RawMessage(`{}`), CreatedAt: now}
		return
	}
	role := s.roles[string(RoleAdmin)]
	role.Mode = string(RoleAdmin)
	role.Permissions = json.RawMessage(`{}`)
	s.roles[string(RoleAdmin)] = role
}

func (s *Store) PrepareBootstrap(ctx context.Context) (BootstrapStatus, error) {
	if err := s.EnsureDefaults(ctx); err != nil {
		return BootstrapStatus{}, err
	}
	return s.GetBootstrapStatus(ctx)
}

func (s *Store) GetBootstrapStatus(ctx context.Context) (BootstrapStatus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return BootstrapStatus{Initialized: s.hasLocalAdminLocked()}, nil
}

func (s *Store) CompleteBootstrap(ctx context.Context, adminUsername, adminPassword string) error {
	adminUsername, err := cleanConfigField("admin username", adminUsername, 160, false)
	if err != nil {
		return err
	}
	adminPassword = strings.TrimSpace(adminPassword)
	if adminPassword == "" {
		return fmt.Errorf("admin password is required")
	}
	passwordHash, err := hashLocalPassword(adminPassword)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.ensureDefaultsLocked(time.Now().UTC())
	if s.hasLocalAdminLocked() {
		s.mu.Unlock()
		return fmt.Errorf("application is already initialized")
	}

	if s.users == nil {
		s.users = make(map[string]ConfigUser)
	}
	if s.sessionVersions == nil {
		s.sessionVersions = make(map[string]int64)
	}
	s.users[adminUsername] = ConfigUser{
		Username:     adminUsername,
		Role:         RoleAdmin,
		PasswordHash: passwordHash,
		CreatedAt:    time.Now().UTC(),
	}
	s.sessionVersions[adminUsername] = nextSessionVersion(s.sessionVersions[adminUsername])
	snapshot := s.snapshotLocked()
	s.mu.Unlock()
	return s.SaveConfigSnapshot(ctx, snapshot)
}

func nextSessionVersion(current int64) int64 {
	if current < 1 {
		return 1
	}
	return current + 1
}

func (s *Store) hasLocalAdminLocked() bool {
	for _, user := range s.users {
		if strings.TrimSpace(user.PasswordHash) == "" {
			continue
		}
		role, ok := s.roles[string(user.Role)]
		if ok && role.Mode == string(RoleAdmin) {
			return true
		}
	}
	return false
}

func (s *Store) getAppState(ctx context.Context, key string) (string, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM app_state WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (s *Store) setAppState(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO app_state (key, value, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, value, time.Now().UTC().Format(time.RFC3339Nano),
	)
	return err
}
