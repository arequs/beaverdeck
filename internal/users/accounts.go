package users

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (s *Store) EnsureAdmin(ctx context.Context, token string) error {
	if strings.TrimSpace(token) == "" {
		return errors.New("ADMIN_TOKEN is not configured")
	}
	passwordHash, err := hashLocalPassword(token)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, _ = s.db.ExecContext(ctx, `INSERT OR IGNORE INTO roles (name, mode, permissions, created_at) VALUES ('admin', 'admin', '{}', ?)`, now)
	_, _ = s.db.ExecContext(ctx, `INSERT OR IGNORE INTO roles (name, mode, permissions, created_at) VALUES ('viewer', 'viewer', '{}', ?)`, now)

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO users (username, token, role, auth_source, google_subject, session_version, created_at)
		 VALUES ('admin', ?, ?, 'local', '', 1, ?)
		 ON CONFLICT(username) DO UPDATE SET
		   token = excluded.token,
		   role = excluded.role,
		   auth_source = 'local',
		   google_subject = '',
		   session_version = CASE WHEN users.token = excluded.token THEN users.session_version ELSE users.session_version + 1 END`,
		passwordHash, string(RoleAdmin), now,
	)
	return err
}

func (s *Store) Authenticate(ctx context.Context, token string) (*UserWithToken, error) {
	if strings.TrimSpace(token) == "" {
		return nil, sql.ErrNoRows
	}
	var (
		user      UserWithToken
		permsRaw  string
		createdAt string
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT u.username, u.role,
		        COALESCE(r.mode, CASE WHEN u.role = 'admin' THEN 'admin' ELSE 'viewer' END) AS mode,
		        COALESCE(r.permissions, '{}') AS permissions,
		        sess.token, u.auth_source, u.session_version, u.created_at
		 FROM users u
		 JOIN sessions sess ON sess.username = u.username
		 LEFT JOIN roles r ON r.name = u.role
		 WHERE sess.token = ?
		   AND sess.session_version = u.session_version`, token,
	).Scan(&user.Username, &user.Role, &user.RoleMode, &permsRaw, &user.Token, &user.AuthSource, &user.SessionVersion, &createdAt)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(permsRaw) == "" {
		permsRaw = "{}"
	}
	user.Permissions = json.RawMessage(permsRaw)
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
	var (
		user      UserWithToken
		permsRaw  string
		storedPwd string
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT u.username, u.role,
		        COALESCE(r.mode, CASE WHEN u.role = 'admin' THEN 'admin' ELSE 'viewer' END) AS mode,
		        COALESCE(r.permissions, '{}') AS permissions,
		        u.token, u.auth_source, u.session_version
		 FROM users u
		 LEFT JOIN roles r ON r.name = u.role
		 WHERE u.username = ?`, username,
	).Scan(&user.Username, &user.Role, &user.RoleMode, &permsRaw, &storedPwd, &user.AuthSource, &user.SessionVersion)
	if err != nil {
		return nil, err
	}
	passwordMatched, needsUpgrade, err := verifyLocalPassword(storedPwd, password)
	if err != nil {
		return nil, err
	}
	if user.AuthSource != "local" || !passwordMatched {
		return nil, sql.ErrNoRows
	}
	if needsUpgrade {
		if passwordHash, hashErr := hashLocalPassword(password); hashErr == nil {
			_, _ = s.db.ExecContext(ctx, `UPDATE users SET token = ? WHERE username = ? AND auth_source = 'local'`, passwordHash, user.Username)
		}
	}
	if strings.TrimSpace(permsRaw) == "" {
		permsRaw = "{}"
	}
	user.Permissions = json.RawMessage(permsRaw)
	return &user, nil
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
	var sessionVersion int64
	if err := s.db.QueryRowContext(ctx, `SELECT session_version FROM users WHERE username = ?`, username).Scan(&sessionVersion); err != nil {
		return "", err
	}
	token, err := randomToken()
	if err != nil {
		return "", err
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO sessions (token, username, auth_source, session_version, created_at) VALUES (?, ?, ?, ?, ?)`,
		token, username, authSource, sessionVersion, time.Now().UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return "", err
	}
	return token, nil
}

func (s *Store) RevokeSession(ctx context.Context, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token = ?`, token)
	return err
}

func (s *Store) InvalidateUserSessions(ctx context.Context, username string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return fmt.Errorf("username is required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, `UPDATE users SET session_version = session_version + 1 WHERE username = ?`, username)
	if err != nil {
		return err
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE username = ?`, username); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) List(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT u.username, u.role, u.auth_source, COALESCE(COUNT(s.token), 0) AS session_count, u.created_at
		 FROM users u
		 LEFT JOIN sessions s ON s.username = u.username AND s.session_version = u.session_version
		 GROUP BY u.username, u.role, u.auth_source, u.created_at
		 ORDER BY u.username ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []User
	for rows.Next() {
		var (
			user      User
			createdAt string
		)
		if err := rows.Scan(&user.Username, &user.Role, &user.AuthSource, &user.SessionCount, &createdAt); err != nil {
			return nil, err
		}
		if parsed, err := time.Parse(time.RFC3339Nano, createdAt); err == nil {
			user.CreatedAt = parsed
		}
		out = append(out, user)
	}
	return out, rows.Err()
}

func (s *Store) Create(ctx context.Context, username, token string, role Role) error {
	username = strings.TrimSpace(username)
	token = strings.TrimSpace(token)
	if username == "" || token == "" {
		return fmt.Errorf("username and password are required")
	}
	if !s.roleExists(ctx, string(role)) {
		return fmt.Errorf("role does not exist: %s", role)
	}
	passwordHash, err := hashLocalPassword(token)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO users (username, token, role, auth_source, google_subject, session_version, created_at) VALUES (?, ?, ?, 'local', '', 1, ?)`,
		username, passwordHash, string(role), time.Now().UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (s *Store) UpdateUserRole(ctx context.Context, username string, role Role) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return fmt.Errorf("username is required")
	}
	if !s.roleExists(ctx, string(role)) {
		return fmt.Errorf("role does not exist: %s", role)
	}
	if username == "admin" && role != RoleAdmin {
		return fmt.Errorf("admin role cannot be changed")
	}

	var authSource string
	if err := s.db.QueryRowContext(ctx, `SELECT auth_source FROM users WHERE username = ?`, username).Scan(&authSource); err != nil {
		return err
	}
	if authSource != "local" {
		return fmt.Errorf("%s user role is managed by external group mapping", authSource)
	}

	res, err := s.db.ExecContext(ctx, `UPDATE users SET role = ? WHERE username = ?`, string(role), username)
	if err != nil {
		return err
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) ResetLocalPassword(ctx context.Context, username, password string) error {
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	if username == "" || password == "" {
		return fmt.Errorf("username and password are required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var authSource string
	if err := tx.QueryRowContext(ctx, `SELECT auth_source FROM users WHERE username = ?`, username).Scan(&authSource); err != nil {
		return err
	}
	if authSource != "local" {
		return fmt.Errorf("password reset is available only for local users")
	}
	passwordHash, err := hashLocalPassword(password)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET token = ?, session_version = session_version + 1 WHERE username = ?`, passwordHash, username); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE username = ?`, username); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) upsertExternalUser(ctx context.Context, username, externalSubject string, role Role, authSource string) error {
	username = strings.TrimSpace(strings.ToLower(username))
	externalSubject = strings.TrimSpace(externalSubject)
	authSource = strings.TrimSpace(strings.ToLower(authSource))
	if username == "" || externalSubject == "" || authSource == "" {
		return fmt.Errorf("external username, subject and auth source are required")
	}
	if !s.roleExists(ctx, string(role)) {
		return fmt.Errorf("role does not exist: %s", role)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var (
		existingUsername string
		existingSource   string
	)
	err = tx.QueryRowContext(ctx,
		`SELECT username, auth_source
			 FROM users
			 WHERE google_subject = ? OR username = ?
			 ORDER BY CASE WHEN google_subject = ? THEN 0 ELSE 1 END
			 LIMIT 1`,
		externalSubject, username, externalSubject,
	).Scan(&existingUsername, &existingSource)
	switch {
	case err == sql.ErrNoRows:
		placeholder, randErr := randomToken()
		if randErr != nil {
			return randErr
		}
		_, err = tx.ExecContext(ctx,
			`INSERT INTO users (username, token, role, auth_source, google_subject, session_version, created_at) VALUES (?, ?, ?, ?, ?, 1, ?)`,
			username, placeholder, string(role), authSource, externalSubject, time.Now().UTC().Format(time.RFC3339Nano),
		)
		if err != nil {
			return fmt.Errorf("create %s user: %w", authSource, err)
		}
	case err != nil:
		return err
	default:
		if existingSource != authSource {
			return fmt.Errorf("user %s already exists as local user", existingUsername)
		}
		if existingUsername != username {
			if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE username = ?`, existingUsername); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `UPDATE users SET username = ?, role = ?, google_subject = ?, auth_source = ?, session_version = session_version + 1 WHERE username = ?`, username, string(role), externalSubject, authSource, existingUsername); err != nil {
				return err
			}
			break
		}
		if _, err := tx.ExecContext(ctx, `UPDATE users SET role = ?, google_subject = ?, auth_source = ? WHERE username = ?`, string(role), externalSubject, authSource, username); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) UpsertGoogleUser(ctx context.Context, username, googleSubject string, role Role) error {
	return s.upsertExternalUser(ctx, username, googleSubject, role, "google")
}

func (s *Store) UpsertOIDCUser(ctx context.Context, username, subject string, role Role) error {
	return s.upsertExternalUser(ctx, username, subject, role, "oidc")
}

func (s *Store) ListRoles(ctx context.Context) ([]RoleDef, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT name, mode, permissions, created_at FROM roles ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RoleDef
	for rows.Next() {
		var (
			role      RoleDef
			perms     string
			createdAt string
		)
		if err := rows.Scan(&role.Name, &role.Mode, &perms, &createdAt); err != nil {
			return nil, err
		}
		role.Permissions = json.RawMessage(perms)
		if parsed, err := time.Parse(time.RFC3339Nano, createdAt); err == nil {
			role.CreatedAt = parsed
		}
		out = append(out, role)
	}
	return out, rows.Err()
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
	if name == string(RoleViewer) && mode != string(RoleViewer) {
		return fmt.Errorf("viewer role mode must stay viewer")
	}
	if len(permissions) == 0 {
		permissions = json.RawMessage(`{}`)
	}
	if !json.Valid(permissions) {
		return fmt.Errorf("permissions must be valid json")
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO roles (name, mode, permissions, created_at) VALUES (?, ?, ?, ?)`,
		name, mode, string(permissions), time.Now().UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("create role: %w", err)
	}
	return nil
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
	if name == string(RoleViewer) && mode != string(RoleViewer) {
		return fmt.Errorf("viewer role mode must stay viewer")
	}
	if len(permissions) == 0 {
		permissions = json.RawMessage(`{}`)
	}
	if !json.Valid(permissions) {
		return fmt.Errorf("permissions must be valid json")
	}
	res, err := s.db.ExecContext(ctx, `UPDATE roles SET mode = ?, permissions = ? WHERE name = ?`, mode, string(permissions), name)
	if err != nil {
		return fmt.Errorf("update role: %w", err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) DeleteRole(ctx context.Context, name string) error {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return fmt.Errorf("role name is required")
	}
	if name == string(RoleAdmin) || name == string(RoleViewer) {
		return fmt.Errorf("default roles cannot be deleted")
	}

	var inUse int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM users WHERE role = ?`, name).Scan(&inUse); err != nil {
		return err
	}
	if inUse > 0 {
		return fmt.Errorf("role is assigned to users")
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM google_group_roles WHERE role = ?`, name).Scan(&inUse); err != nil {
		return err
	}
	if inUse > 0 {
		return fmt.Errorf("role is assigned to google group mappings")
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM oidc_group_roles WHERE role = ?`, name).Scan(&inUse); err != nil {
		return err
	}
	if inUse > 0 {
		return fmt.Errorf("role is assigned to OpenID Connect group mappings")
	}

	res, err := s.db.ExecContext(ctx, `DELETE FROM roles WHERE name = ?`, name)
	if err != nil {
		return err
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) roleExists(ctx context.Context, name string) bool {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM roles WHERE name = ?`, strings.TrimSpace(strings.ToLower(name))).Scan(&count)
	return err == nil && count > 0
}

func (s *Store) Delete(ctx context.Context, username string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return fmt.Errorf("username is required")
	}
	if username == "admin" {
		return fmt.Errorf("admin user cannot be deleted")
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE username = ?`, username); err != nil {
		return err
	}

	res, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE username = ?`, username)
	if err != nil {
		return err
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}
	return nil
}
