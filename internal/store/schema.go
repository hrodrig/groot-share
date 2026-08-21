package store

import (
	"context"
	"database/sql"
	"fmt"
)

const schemaSQL = `
CREATE TABLE IF NOT EXISTS users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  username TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  password_hash TEXT NOT NULL,
  role TEXT NOT NULL DEFAULT 'uploader' CHECK(role IN ('viewer','uploader','admin')),
  active INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE TABLE IF NOT EXISTS sessions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL REFERENCES users(id),
  token_hash TEXT NOT NULL UNIQUE,
  expires_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS api_keys (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL REFERENCES users(id),
  key_hash TEXT NOT NULL UNIQUE,
  prefix TEXT NOT NULL,
  scope TEXT NOT NULL DEFAULT 'upload' CHECK(scope IN ('upload','read')),
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  last_used_at TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS archives (
  id TEXT PRIMARY KEY,
  key TEXT NOT NULL,
  size INTEGER NOT NULL,
  sha256 TEXT NOT NULL,
  source TEXT NOT NULL,
  uploaded_by INTEGER REFERENCES users(id),
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE TABLE IF NOT EXISTS transit (
  id TEXT PRIMARY KEY,
  key TEXT NOT NULL,
  s3_key TEXT NOT NULL UNIQUE,
  size INTEGER NOT NULL,
  sha256 TEXT NOT NULL,
  path TEXT NOT NULL,
  uploaded_by INTEGER REFERENCES users(id),
  created_at TEXT NOT NULL,
  last_error TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS audit (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  actor TEXT NOT NULL,
  actor_id INTEGER NOT NULL DEFAULT 0,
  action TEXT NOT NULL,
  object_id TEXT NOT NULL DEFAULT '',
  object_key TEXT NOT NULL DEFAULT '',
  remote_ip TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS share_links (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  archive_id TEXT NOT NULL,
  token_hash TEXT NOT NULL UNIQUE,
  created_by INTEGER NOT NULL REFERENCES users(id),
  label TEXT NOT NULL DEFAULT '',
  max_uses INTEGER NOT NULL DEFAULT 0,
  use_count INTEGER NOT NULL DEFAULT 0,
  expires_at TEXT NOT NULL DEFAULT '',
  revoked_at TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE TABLE IF NOT EXISTS archive_pins (
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  archive_id TEXT NOT NULL,
  archive_key TEXT NOT NULL DEFAULT '',
  archive_size INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  PRIMARY KEY (user_id, archive_id)
);
CREATE INDEX IF NOT EXISTS archives_sha256_idx ON archives(sha256);
CREATE INDEX IF NOT EXISTS transit_sha256_idx ON transit(sha256);
CREATE INDEX IF NOT EXISTS share_links_archive_idx ON share_links(archive_id);
CREATE INDEX IF NOT EXISTS archive_pins_user_idx ON archive_pins(user_id, created_at DESC);
`

func (s *Store) migrate() error {
	if _, err := s.db.Exec(schemaSQL); err != nil {
		return err
	}
	var ver int
	if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&ver); err != nil {
		return fmt.Errorf("user_version: %w", err)
	}
	if ver >= 1 {
		if err := s.ensureAPIKeyLastUsed(); err != nil {
			return err
		}
		return s.ensureUserName()
	}
	if hasColumn(s.db, "users", "admin") {
		if err := migrateUsersAdminToRole(s.db); err != nil {
			return err
		}
	}
	if !hasColumn(s.db, "api_keys", "scope") {
		if _, err := s.db.Exec(`ALTER TABLE api_keys ADD COLUMN scope TEXT NOT NULL DEFAULT 'upload'`); err != nil {
			return fmt.Errorf("add api_keys.scope: %w", err)
		}
	}
	if _, err := s.db.Exec(`PRAGMA user_version = 1`); err != nil {
		return fmt.Errorf("set user_version: %w", err)
	}
	if err := s.ensureAPIKeyLastUsed(); err != nil {
		return err
	}
	return s.ensureUserName()
}

func (s *Store) ensureUserName() error {
	if !hasColumn(s.db, "users", "name") {
		if _, err := s.db.Exec(`ALTER TABLE users ADD COLUMN name TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("add users.name: %w", err)
		}
	}
	if _, err := s.db.Exec(`UPDATE users SET name = username WHERE name = ''`); err != nil {
		return fmt.Errorf("backfill users.name: %w", err)
	}
	return nil
}

func (s *Store) ensureAPIKeyLastUsed() error {
	if hasColumn(s.db, "api_keys", "last_used_at") {
		return nil
	}
	if _, err := s.db.Exec(`ALTER TABLE api_keys ADD COLUMN last_used_at TEXT NOT NULL DEFAULT ''`); err != nil {
		return fmt.Errorf("add api_keys.last_used_at: %w", err)
	}
	return nil
}

func hasColumn(db *sql.DB, table, col string) bool {
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return false
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return false
		}
		if name == col {
			return true
		}
	}
	return false
}

func migrateUsersAdminToRole(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`
		CREATE TABLE users_new (
		  id INTEGER PRIMARY KEY AUTOINCREMENT,
		  username TEXT NOT NULL UNIQUE,
		  password_hash TEXT NOT NULL,
		  role TEXT NOT NULL DEFAULT 'uploader' CHECK(role IN ('viewer','uploader','admin')),
		  active INTEGER NOT NULL DEFAULT 1,
		  created_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`); err != nil {
		return fmt.Errorf("users_new: %w", err)
	}
	if _, err := tx.Exec(`
		INSERT INTO users_new (id, username, password_hash, role, active, created_at)
		SELECT id, username, password_hash,
		       CASE WHEN admin = 1 THEN 'admin' ELSE 'uploader' END,
		       1, created_at
		FROM users`); err != nil {
		return fmt.Errorf("copy users: %w", err)
	}
	if _, err := tx.Exec(`DROP TABLE users`); err != nil {
		return fmt.Errorf("drop users: %w", err)
	}
	if _, err := tx.Exec(`ALTER TABLE users_new RENAME TO users`); err != nil {
		return fmt.Errorf("rename users: %w", err)
	}
	return tx.Commit()
}

// migrateLegacyV0 creates a pre-RBAC schema for migration tests.
func (s *Store) migrateLegacyV0(ctx context.Context) error {
	if _, err := s.db.Exec(`PRAGMA user_version = 0`); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `
		DROP TABLE IF EXISTS users;
		CREATE TABLE users (
		  id INTEGER PRIMARY KEY AUTOINCREMENT,
		  username TEXT NOT NULL UNIQUE,
		  password_hash TEXT NOT NULL,
		  admin INTEGER NOT NULL DEFAULT 0,
		  created_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`)
	return err
}
