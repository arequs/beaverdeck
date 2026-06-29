package users

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

func Open(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	dbPath := filepath.Join(dataDir, "users.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable sqlite foreign keys: %w", err)
	}
	if _, err := db.Exec(`PRAGMA secure_delete = ON`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable sqlite secure delete: %w", err)
	}

	if _, err := db.Exec(`
CREATE TABLE IF NOT EXISTS app_state (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL DEFAULT ''
);`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate app_state sqlite: %w", err)
	}
	var authTableCount int
	if err := db.QueryRow(`
SELECT COUNT(1)
  FROM sqlite_master
 WHERE type = 'table'
   AND name IN ('users', 'roles', 'google_config', 'google_group_roles', 'oidc_config', 'oidc_group_roles', 'sessions', 'external_sessions')`).Scan(&authTableCount); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("inspect local auth storage sqlite: %w", err)
	}
	for _, stmt := range []string{
		`DROP TABLE IF EXISTS sessions`,
		`DROP TABLE IF EXISTS external_sessions`,
		`DROP TABLE IF EXISTS google_group_roles`,
		`DROP TABLE IF EXISTS oidc_group_roles`,
		`DROP TABLE IF EXISTS users`,
		`DROP TABLE IF EXISTS roles`,
		`DROP TABLE IF EXISTS google_config`,
		`DROP TABLE IF EXISTS oidc_config`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("remove local auth storage sqlite: %w", err)
		}
	}
	if authTableCount > 0 {
		if _, err := db.Exec(`VACUUM`); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("vacuum local auth storage sqlite: %w", err)
		}
	}

	sessionSigningKey, err := randomToken()
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	store := &Store{db: db, sessionSigningKey: []byte(sessionSigningKey)}
	store.applyConfigSnapshotLocked(defaultConfigSnapshot(time.Now().UTC()))
	return store, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}
