package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hrodrig/groot-share/internal/auth"
)

// ErrNotFound is a missing row.
var ErrNotFound = errors.New("not found")

// User is an identity row.
type User struct {
	ID           int64
	Username     string
	PasswordHash string
	Role         auth.Role
	Active       bool
}

// APIKeyAuth is the user and scope for a presented api_key hash.
type APIKeyAuth struct {
	User  User
	Scope auth.KeyScope
}

// UserCount returns how many users exist.
func (s *Store) UserCount(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}
	return n, nil
}

// CreateUser inserts a user. passwordHash is already bcrypt.
func (s *Store) CreateUser(ctx context.Context, username, passwordHash string, role auth.Role) (User, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return User{}, fmt.Errorf("username is required")
	}
	if !auth.ValidRole(role) {
		return User{}, fmt.Errorf("invalid role")
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO users (username, password_hash, role, active) VALUES (?, ?, ?, 1)`,
		username, passwordHash, string(role),
	)
	if err != nil {
		return User{}, fmt.Errorf("insert user: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return User{}, fmt.Errorf("user id: %w", err)
	}
	return User{ID: id, Username: username, PasswordHash: passwordHash, Role: role, Active: true}, nil
}

// UserByUsername looks up a login name.
func (s *Store) UserByUsername(ctx context.Context, username string) (User, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, username, password_hash, role, active FROM users WHERE username = ?`,
		strings.TrimSpace(username),
	)
	return scanUser(row)
}

// UserByID loads a user by primary key.
func (s *Store) UserByID(ctx context.Context, id int64) (User, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, username, password_hash, role, active FROM users WHERE id = ?`,
		id,
	)
	return scanUser(row)
}

// EnsureAdmin creates the first admin when the table is empty.
func (s *Store) EnsureAdmin(ctx context.Context, username, password string) error {
	n, err := s.UserCount(ctx)
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return fmt.Errorf("GFS_BOOTSTRAP_ADMIN and GFS_BOOTSTRAP_PASSWORD are required when the user table is empty (fail closed)")
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return fmt.Errorf("bootstrap password: %w", err)
	}
	_, err = s.CreateUser(ctx, username, hash, auth.RoleAdmin)
	if err != nil {
		return fmt.Errorf("bootstrap admin: %w", err)
	}
	return nil
}

// CreateSession stores a hashed session token.
func (s *Store) CreateSession(ctx context.Context, userID int64, tokenHash string, expires time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (user_id, token_hash, expires_at) VALUES (?, ?, ?)`,
		userID, tokenHash, expires.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}
	return nil
}

// UserBySessionHash returns the user for a still-valid session.
func (s *Store) UserBySessionHash(ctx context.Context, tokenHash string) (User, error) {
	var u User
	var role string
	var active int
	var exp string
	err := s.db.QueryRowContext(ctx, `
		SELECT u.id, u.username, u.password_hash, u.role, u.active, s.expires_at
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = ?`, tokenHash,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &role, &active, &exp)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("session lookup: %w", err)
	}
	until, err := time.Parse(time.RFC3339, exp)
	if err != nil || time.Now().After(until) {
		return User{}, ErrNotFound
	}
	u.Role = auth.Role(role)
	u.Active = active != 0
	if !u.Active {
		return User{}, ErrNotFound
	}
	return u, nil
}

// DeleteSession removes one session by token hash.
func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// CreateAPIKey stores a hashed key with scope.
func (s *Store) CreateAPIKey(ctx context.Context, userID int64, keyHash, prefix string, scope auth.KeyScope) error {
	if !auth.ValidKeyScope(scope) {
		return fmt.Errorf("invalid api_key scope")
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO api_keys (user_id, key_hash, prefix, scope) VALUES (?, ?, ?, ?)`,
		userID, keyHash, prefix, string(scope),
	)
	if err != nil {
		return fmt.Errorf("insert api key: %w", err)
	}
	return nil
}

// AuthByAPIKeyHash looks up the owner and scope of a key hash.
func (s *Store) AuthByAPIKeyHash(ctx context.Context, keyHash string) (APIKeyAuth, error) {
	var u User
	var role, scope string
	var active int
	err := s.db.QueryRowContext(ctx, `
		SELECT u.id, u.username, u.password_hash, u.role, u.active, k.scope
		FROM api_keys k
		JOIN users u ON u.id = k.user_id
		WHERE k.key_hash = ?`, keyHash,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &role, &active, &scope)
	if errors.Is(err, sql.ErrNoRows) {
		return APIKeyAuth{}, ErrNotFound
	}
	if err != nil {
		return APIKeyAuth{}, fmt.Errorf("api key lookup: %w", err)
	}
	u.Role = auth.Role(role)
	u.Active = active != 0
	if !u.Active {
		return APIKeyAuth{}, ErrNotFound
	}
	return APIKeyAuth{User: u, Scope: auth.KeyScope(scope)}, nil
}

// APIKeyHashStored reports whether this hash exists (tests).
func (s *Store) APIKeyHashStored(ctx context.Context, keyHash string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM api_keys WHERE key_hash = ?`, keyHash).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func scanUser(row *sql.Row) (User, error) {
	var u User
	var role string
	var active int
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &role, &active)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("get user: %w", err)
	}
	u.Role = auth.Role(role)
	u.Active = active != 0
	return u, nil
}
