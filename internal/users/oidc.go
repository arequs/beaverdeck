package users

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

func (s *Store) GetOIDCConfig(ctx context.Context) (OIDCConfig, error) {
	var cfg OIDCConfig
	err := s.db.QueryRowContext(ctx,
		`SELECT provider_name, issuer_url, client_id, client_secret, scopes, hosted_domain, email_claim, groups_claim, updated_at
		 FROM oidc_config WHERE singleton = 1`,
	).Scan(&cfg.ProviderName, &cfg.IssuerURL, &cfg.ClientID, &cfg.ClientSecret, &cfg.Scopes, &cfg.HostedDomain, &cfg.EmailClaim, &cfg.GroupsClaim, &cfg.UpdatedAt)
	return cfg, err
}

func (s *Store) UpdateOIDCConfig(ctx context.Context, cfg OIDCConfig) error {
	scopes := strings.TrimSpace(cfg.Scopes)
	if scopes == "" {
		scopes = "openid email profile groups"
	}
	emailClaim := strings.TrimSpace(cfg.EmailClaim)
	if emailClaim == "" {
		emailClaim = "email"
	}
	groupsClaim := strings.TrimSpace(cfg.GroupsClaim)
	if groupsClaim == "" {
		groupsClaim = "groups"
	}
	providerName := strings.TrimSpace(cfg.ProviderName)
	if providerName == "" {
		providerName = "OpenID Connect"
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE oidc_config
		 SET provider_name = ?, issuer_url = ?, client_id = ?, client_secret = ?, scopes = ?, hosted_domain = ?, email_claim = ?, groups_claim = ?, updated_at = ?
		 WHERE singleton = 1`,
		providerName,
		strings.TrimSpace(cfg.IssuerURL),
		strings.TrimSpace(cfg.ClientID),
		strings.TrimSpace(cfg.ClientSecret),
		scopes,
		strings.TrimSpace(strings.ToLower(cfg.HostedDomain)),
		emailClaim,
		groupsClaim,
		time.Now().UTC().Format(time.RFC3339Nano),
	)
	return err
}

func (s *Store) ListOIDCGroupRoles(ctx context.Context) ([]OIDCGroupRole, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT group_name, role, created_at
		 FROM oidc_group_roles
		 ORDER BY group_name ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []OIDCGroupRole
	for rows.Next() {
		var (
			item      OIDCGroupRole
			createdAt string
		)
		if err := rows.Scan(&item.GroupName, &item.Role, &createdAt); err != nil {
			return nil, err
		}
		if parsed, err := time.Parse(time.RFC3339Nano, createdAt); err == nil {
			item.CreatedAt = parsed
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) UpsertOIDCGroupRole(ctx context.Context, groupName string, role Role) error {
	groupName = strings.TrimSpace(groupName)
	if groupName == "" {
		return fmt.Errorf("OpenID Connect group is required")
	}
	if !s.roleExists(ctx, string(role)) {
		return fmt.Errorf("role does not exist: %s", role)
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO oidc_group_roles (group_name, role, created_at)
		 VALUES (?, ?, ?)
		 ON CONFLICT(group_name) DO UPDATE SET role = excluded.role`,
		groupName, string(role), time.Now().UTC().Format(time.RFC3339Nano),
	)
	return err
}

func (s *Store) DeleteOIDCGroupRole(ctx context.Context, groupName string) error {
	groupName = strings.TrimSpace(groupName)
	if groupName == "" {
		return fmt.Errorf("OpenID Connect group is required")
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM oidc_group_roles WHERE group_name = ?`, groupName)
	if err != nil {
		return err
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) ResetOIDCAuth(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`UPDATE oidc_config
		 SET provider_name = '', issuer_url = '', client_id = '', client_secret = '', scopes = 'openid email profile groups', hosted_domain = '', email_claim = 'email', groups_claim = 'groups', updated_at = ?
		 WHERE singleton = 1`,
		time.Now().UTC().Format(time.RFC3339Nano),
	); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM oidc_group_roles`); err != nil {
		return err
	}
	return tx.Commit()
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

	rows, err := s.db.QueryContext(ctx,
		`SELECT m.group_name, m.role,
		        COALESCE(r.mode, CASE WHEN m.role = 'admin' THEN 'admin' ELSE 'viewer' END) AS mode
		 FROM oidc_group_roles m
		 LEFT JOIN roles r ON r.name = m.role
		 ORDER BY CASE WHEN COALESCE(r.mode, CASE WHEN m.role = 'admin' THEN 'admin' ELSE 'viewer' END) = 'admin' THEN 0 ELSE 1 END,
		          m.role ASC,
		          m.group_name ASC`,
	)
	if err != nil {
		return "", "", err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			groupName string
			role      Role
			mode      string
		)
		if err := rows.Scan(&groupName, &role, &mode); err != nil {
			return "", "", err
		}
		if _, ok := normalized[strings.ToLower(strings.TrimSpace(groupName))]; !ok {
			continue
		}
		return role, groupName, nil
	}
	if err := rows.Err(); err != nil {
		return "", "", err
	}
	return "", "", fmt.Errorf("OpenID Connect user is not a member of any mapped group")
}
