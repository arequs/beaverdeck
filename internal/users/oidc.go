package users

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
)

func (s *Store) GetOIDCConfig(ctx context.Context) (OIDCConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.oidcConfig, nil
}

func (s *Store) UpdateOIDCConfig(ctx context.Context, cfg OIDCConfig) error {
	scopes := strings.TrimSpace(cfg.Scopes)
	if scopes == "" {
		scopes = defaultOIDCScopes
	}
	emailClaim := strings.TrimSpace(cfg.EmailClaim)
	if emailClaim == "" {
		emailClaim = defaultOIDCEmailClaim
	}
	groupsClaim := strings.TrimSpace(cfg.GroupsClaim)
	if groupsClaim == "" {
		groupsClaim = defaultOIDCGroupsClaim
	}
	providerName := strings.TrimSpace(cfg.ProviderName)
	if providerName == "" {
		providerName = "OpenID Connect"
	}

	return s.mutateConfigAndPersist(ctx, func() error {
		s.oidcConfig = OIDCConfig{
			ProviderName: providerName,
			IssuerURL:    strings.TrimSpace(cfg.IssuerURL),
			ClientID:     strings.TrimSpace(cfg.ClientID),
			ClientSecret: strings.TrimSpace(cfg.ClientSecret),
			Scopes:       scopes,
			HostedDomain: strings.TrimSpace(strings.ToLower(cfg.HostedDomain)),
			EmailClaim:   emailClaim,
			GroupsClaim:  groupsClaim,
			UpdatedAt:    time.Now().UTC().Format(time.RFC3339Nano),
		}
		return nil
	})
}

func (s *Store) ListOIDCGroupRoles(ctx context.Context) ([]OIDCGroupRole, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]OIDCGroupRole, 0, len(s.oidcMappings))
	for _, item := range s.oidcMappings {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].GroupName) < strings.ToLower(out[j].GroupName)
	})
	return out, nil
}

func (s *Store) UpsertOIDCGroupRole(ctx context.Context, groupName string, role Role) error {
	groupName = strings.TrimSpace(groupName)
	role = Role(strings.TrimSpace(strings.ToLower(string(role))))
	if groupName == "" {
		return fmt.Errorf("OpenID Connect group is required")
	}
	key := strings.ToLower(groupName)

	return s.mutateConfigAndPersist(ctx, func() error {
		if !s.roleExistsLocked(string(role)) {
			return fmt.Errorf("role does not exist: %s", role)
		}
		if s.oidcMappings == nil {
			s.oidcMappings = make(map[string]OIDCGroupRole)
		}
		item, exists := s.oidcMappings[key]
		if !exists {
			item = OIDCGroupRole{GroupName: groupName, CreatedAt: time.Now().UTC()}
		}
		item.GroupName = groupName
		item.Role = role
		s.oidcMappings[key] = item
		return nil
	})
}

func (s *Store) DeleteOIDCGroupRole(ctx context.Context, groupName string) error {
	groupName = strings.TrimSpace(groupName)
	if groupName == "" {
		return fmt.Errorf("OpenID Connect group is required")
	}
	key := strings.ToLower(groupName)

	return s.mutateConfigAndPersist(ctx, func() error {
		if _, ok := s.oidcMappings[key]; !ok {
			return sql.ErrNoRows
		}
		delete(s.oidcMappings, key)
		return nil
	})
}

func (s *Store) ResetOIDCAuth(ctx context.Context) error {
	return s.mutateConfigAndPersist(ctx, func() error {
		s.oidcConfig = OIDCConfig{
			ProviderName: "OpenID Connect",
			Scopes:       defaultOIDCScopes,
			EmailClaim:   defaultOIDCEmailClaim,
			GroupsClaim:  defaultOIDCGroupsClaim,
			UpdatedAt:    time.Now().UTC().Format(time.RFC3339Nano),
		}
		s.oidcMappings = make(map[string]OIDCGroupRole)
		return nil
	})
}

func (s *Store) ResolveOIDCRole(ctx context.Context, groups []string) (Role, string, error) {
	normalized := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		group = strings.ToLower(strings.TrimSpace(group))
		if group != "" {
			normalized[group] = struct{}{}
		}
	}
	if len(normalized) == 0 {
		return "", "", fmt.Errorf("OpenID Connect user is not a member of any mapped group")
	}

	s.mu.RLock()
	mappings := make([]OIDCGroupRole, 0, len(s.oidcMappings))
	for _, mapping := range s.oidcMappings {
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
		return strings.ToLower(mappings[i].GroupName) < strings.ToLower(mappings[j].GroupName)
	})
	for _, mapping := range mappings {
		if _, ok := normalized[strings.ToLower(strings.TrimSpace(mapping.GroupName))]; ok {
			return mapping.Role, mapping.GroupName, nil
		}
	}
	return "", "", fmt.Errorf("OpenID Connect user is not a member of any mapped group")
}
