package users

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
)

func (s *Store) GetGoogleConfig(ctx context.Context) (GoogleConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.googleConfig, nil
}

func (s *Store) UpdateGoogleConfig(ctx context.Context, cfg GoogleConfig) error {
	s.mu.Lock()
	s.googleConfig = GoogleConfig{
		ClientID:            strings.TrimSpace(cfg.ClientID),
		ClientSecret:        strings.TrimSpace(cfg.ClientSecret),
		HostedDomain:        strings.TrimSpace(strings.ToLower(cfg.HostedDomain)),
		ServiceAccountJSON:  strings.TrimSpace(cfg.ServiceAccountJSON),
		DelegatedAdminEmail: strings.TrimSpace(strings.ToLower(cfg.DelegatedAdminEmail)),
		UpdatedAt:           time.Now().UTC().Format(time.RFC3339Nano),
	}
	snapshot := s.snapshotLocked()
	s.mu.Unlock()
	return s.SaveConfigSnapshot(ctx, snapshot)
}

func (s *Store) ListGoogleGroupRoles(ctx context.Context) ([]GoogleGroupRole, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]GoogleGroupRole, 0, len(s.googleMappings))
	for _, item := range s.googleMappings {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].GroupEmail < out[j].GroupEmail })
	return out, nil
}

func (s *Store) UpsertGoogleGroupRole(ctx context.Context, groupEmail string, role Role) error {
	groupEmail = strings.TrimSpace(strings.ToLower(groupEmail))
	role = Role(strings.TrimSpace(strings.ToLower(string(role))))
	if groupEmail == "" {
		return fmt.Errorf("google group email is required")
	}

	s.mu.Lock()
	if !s.roleExistsLocked(string(role)) {
		s.mu.Unlock()
		return fmt.Errorf("role does not exist: %s", role)
	}
	if s.googleMappings == nil {
		s.googleMappings = make(map[string]GoogleGroupRole)
	}
	item, exists := s.googleMappings[groupEmail]
	if !exists {
		item = GoogleGroupRole{GroupEmail: groupEmail, CreatedAt: time.Now().UTC()}
	}
	item.Role = role
	s.googleMappings[groupEmail] = item
	snapshot := s.snapshotLocked()
	s.mu.Unlock()
	return s.SaveConfigSnapshot(ctx, snapshot)
}

func (s *Store) DeleteGoogleGroupRole(ctx context.Context, groupEmail string) error {
	groupEmail = strings.TrimSpace(strings.ToLower(groupEmail))
	if groupEmail == "" {
		return fmt.Errorf("google group email is required")
	}

	s.mu.Lock()
	if _, ok := s.googleMappings[groupEmail]; !ok {
		s.mu.Unlock()
		return sql.ErrNoRows
	}
	delete(s.googleMappings, groupEmail)
	snapshot := s.snapshotLocked()
	s.mu.Unlock()
	return s.SaveConfigSnapshot(ctx, snapshot)
}

func (s *Store) ResetGoogleAuth(ctx context.Context) error {
	s.mu.Lock()
	s.googleConfig = GoogleConfig{UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	s.googleMappings = make(map[string]GoogleGroupRole)
	snapshot := s.snapshotLocked()
	s.mu.Unlock()
	return s.SaveConfigSnapshot(ctx, snapshot)
}

func (s *Store) ResolveGoogleRole(ctx context.Context, groups []string) (Role, string, error) {
	normalized := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		group = strings.TrimSpace(strings.ToLower(group))
		if group != "" {
			normalized[group] = struct{}{}
		}
	}
	if len(normalized) == 0 {
		return "", "", fmt.Errorf("google user is not a member of any mapped group")
	}

	s.mu.RLock()
	mappings := make([]GoogleGroupRole, 0, len(s.googleMappings))
	for _, mapping := range s.googleMappings {
		mappings = append(mappings, mapping)
	}
	roleDefs := make(map[string]RoleDef, len(s.roles))
	for name, role := range s.roles {
		roleDefs[name] = role
	}
	s.mu.RUnlock()

	sort.Slice(mappings, func(i, j int) bool {
		leftRole := roleDefs[string(mappings[i].Role)]
		rightRole := roleDefs[string(mappings[j].Role)]
		if (leftRole.Mode == string(RoleAdmin)) != (rightRole.Mode == string(RoleAdmin)) {
			return leftRole.Mode == string(RoleAdmin)
		}
		if mappings[i].Role != mappings[j].Role {
			return mappings[i].Role < mappings[j].Role
		}
		return mappings[i].GroupEmail < mappings[j].GroupEmail
	})
	for _, mapping := range mappings {
		if _, ok := normalized[strings.ToLower(mapping.GroupEmail)]; ok {
			return mapping.Role, mapping.GroupEmail, nil
		}
	}
	return "", "", fmt.Errorf("google user is not a member of any mapped group")
}
