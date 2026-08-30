package users

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
)

type oidcProvider string

const (
	oidcProviderGeneric oidcProvider = "oidc"
	oidcProviderEntra   oidcProvider = "entra"
)

func (s *Store) GetOIDCConfig(ctx context.Context) (OIDCConfig, error) {
	return s.getOIDCConfig(oidcProviderGeneric)
}

func (s *Store) GetEntraConfig(ctx context.Context) (OIDCConfig, error) {
	return s.getOIDCConfig(oidcProviderEntra)
}

func (s *Store) getOIDCConfig(provider oidcProvider) (OIDCConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if provider == oidcProviderEntra {
		return s.entraConfig, nil
	}
	return s.oidcConfig, nil
}

func (s *Store) UpdateOIDCConfig(ctx context.Context, cfg OIDCConfig) error {
	return s.updateOIDCConfig(ctx, oidcProviderGeneric, cfg)
}

func (s *Store) UpdateEntraConfig(ctx context.Context, cfg OIDCConfig) error {
	return s.updateOIDCConfig(ctx, oidcProviderEntra, cfg)
}

func (s *Store) updateOIDCConfig(ctx context.Context, provider oidcProvider, cfg OIDCConfig) error {
	defaultProviderName := "OpenID Connect"
	defaultScopes := defaultOIDCScopes
	if provider == oidcProviderEntra {
		defaultProviderName = "Azure Entra ID"
		defaultScopes = defaultEntraScopes
	}
	scopes := strings.TrimSpace(cfg.Scopes)
	if scopes == "" {
		scopes = defaultScopes
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
		providerName = defaultProviderName
	}
	normalized := OIDCConfig{
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

	return s.mutateConfigAndPersist(ctx, func() error {
		if provider == oidcProviderEntra {
			s.entraConfig = normalized
		} else {
			s.oidcConfig = normalized
		}
		return nil
	})
}

func (s *Store) ListOIDCGroupRoles(ctx context.Context) ([]OIDCGroupRole, error) {
	return s.listOIDCGroupRoles(oidcProviderGeneric)
}

func (s *Store) ListEntraGroupRoles(ctx context.Context) ([]OIDCGroupRole, error) {
	return s.listOIDCGroupRoles(oidcProviderEntra)
}

func (s *Store) listOIDCGroupRoles(provider oidcProvider) ([]OIDCGroupRole, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	mappings := s.oidcMappings
	if provider == oidcProviderEntra {
		mappings = s.entraMappings
	}
	out := make([]OIDCGroupRole, 0, len(mappings))
	for _, item := range mappings {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].GroupName) < strings.ToLower(out[j].GroupName)
	})
	return out, nil
}

func (s *Store) UpsertOIDCGroupRole(ctx context.Context, groupName string, role Role) error {
	return s.upsertOIDCGroupRole(ctx, oidcProviderGeneric, groupName, role)
}

func (s *Store) UpsertEntraGroupRole(ctx context.Context, groupName string, role Role) error {
	return s.upsertOIDCGroupRole(ctx, oidcProviderEntra, groupName, role)
}

func (s *Store) upsertOIDCGroupRole(ctx context.Context, provider oidcProvider, groupName string, role Role) error {
	groupName = strings.TrimSpace(groupName)
	role = Role(strings.TrimSpace(strings.ToLower(string(role))))
	if groupName == "" {
		return fmt.Errorf("%s group is required", oidcProviderLabel(provider))
	}
	key := strings.ToLower(groupName)

	return s.mutateConfigAndPersist(ctx, func() error {
		if !s.roleExistsLocked(string(role)) {
			return fmt.Errorf("role does not exist: %s", role)
		}
		mappings := s.oidcMappings
		if provider == oidcProviderEntra {
			mappings = s.entraMappings
		}
		if mappings == nil {
			mappings = make(map[string]OIDCGroupRole)
		}
		item, exists := mappings[key]
		if !exists {
			item = OIDCGroupRole{GroupName: groupName, CreatedAt: time.Now().UTC()}
		}
		item.GroupName = groupName
		item.Role = role
		mappings[key] = item
		if provider == oidcProviderEntra {
			s.entraMappings = mappings
		} else {
			s.oidcMappings = mappings
		}
		return nil
	})
}

func (s *Store) DeleteOIDCGroupRole(ctx context.Context, groupName string) error {
	return s.deleteOIDCGroupRole(ctx, oidcProviderGeneric, groupName)
}

func (s *Store) DeleteEntraGroupRole(ctx context.Context, groupName string) error {
	return s.deleteOIDCGroupRole(ctx, oidcProviderEntra, groupName)
}

func (s *Store) deleteOIDCGroupRole(ctx context.Context, provider oidcProvider, groupName string) error {
	groupName = strings.TrimSpace(groupName)
	if groupName == "" {
		return fmt.Errorf("%s group is required", oidcProviderLabel(provider))
	}
	key := strings.ToLower(groupName)

	return s.mutateConfigAndPersist(ctx, func() error {
		mappings := s.oidcMappings
		if provider == oidcProviderEntra {
			mappings = s.entraMappings
		}
		if _, ok := mappings[key]; !ok {
			return sql.ErrNoRows
		}
		delete(mappings, key)
		return nil
	})
}

func (s *Store) ResetOIDCAuth(ctx context.Context) error {
	return s.resetOIDCAuth(ctx, oidcProviderGeneric)
}

func (s *Store) ResetEntraAuth(ctx context.Context) error {
	return s.resetOIDCAuth(ctx, oidcProviderEntra)
}

func (s *Store) resetOIDCAuth(ctx context.Context, provider oidcProvider) error {
	return s.mutateConfigAndPersist(ctx, func() error {
		cfg := OIDCConfig{
			ProviderName: "OpenID Connect",
			Scopes:       defaultOIDCScopes,
			EmailClaim:   defaultOIDCEmailClaim,
			GroupsClaim:  defaultOIDCGroupsClaim,
			UpdatedAt:    time.Now().UTC().Format(time.RFC3339Nano),
		}
		mappings := make(map[string]OIDCGroupRole)
		if provider == oidcProviderEntra {
			cfg.ProviderName = "Azure Entra ID"
			cfg.Scopes = defaultEntraScopes
			s.entraConfig = cfg
			s.entraMappings = mappings
		} else {
			s.oidcConfig = cfg
			s.oidcMappings = mappings
		}
		return nil
	})
}

func (s *Store) ResolveOIDCRole(ctx context.Context, groups []string) (Role, string, error) {
	return s.resolveOIDCRole(oidcProviderGeneric, groups)
}

func (s *Store) ResolveEntraRole(ctx context.Context, groups []string) (Role, string, error) {
	return s.resolveOIDCRole(oidcProviderEntra, groups)
}

func (s *Store) resolveOIDCRole(provider oidcProvider, groups []string) (Role, string, error) {
	normalized := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		group = strings.ToLower(strings.TrimSpace(group))
		if group != "" {
			normalized[group] = struct{}{}
		}
	}
	if len(normalized) == 0 {
		return "", "", fmt.Errorf("%s user is not a member of any mapped group", oidcProviderLabel(provider))
	}

	s.mu.RLock()
	providerMappings := s.oidcMappings
	if provider == oidcProviderEntra {
		providerMappings = s.entraMappings
	}
	mappings := make([]OIDCGroupRole, 0, len(providerMappings))
	for _, mapping := range providerMappings {
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
	return "", "", fmt.Errorf("%s user is not a member of any mapped group", oidcProviderLabel(provider))
}

func oidcProviderLabel(provider oidcProvider) string {
	if provider == oidcProviderEntra {
		return "Azure Entra ID"
	}
	return "OpenID Connect"
}
