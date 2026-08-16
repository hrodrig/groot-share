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

// ErrLastAdmin blocks removing or demoting the only active admin.
var ErrLastAdmin = errors.New("last admin")

// UserRecord is a user row including metadata for APIs.
type UserRecord struct {
	User
	CreatedAt time.Time
}

// User is an identity row.
type User struct {
	ID           int64
	Username     string
	Name         string
	PasswordHash string
	Role         auth.Role
	Active       bool
}

// APIKeyAuth is the user and scope for a presented api_key hash.
type APIKeyAuth struct {
	KeyID int64
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

// CreateUser inserts a user. passwordHash is already bcrypt. name is required.
func (s *Store) CreateUser(ctx context.Context, username, name, passwordHash string, role auth.Role) (User, error) {
	username, err := NormalizeUsername(username)
	if err != nil {
		return User{}, err
	}
	name, err = NormalizeName(name)
	if err != nil {
		return User{}, err
	}
	if !auth.ValidRole(role) {
		return User{}, fmt.Errorf("invalid role")
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO users (username, name, password_hash, role, active) VALUES (?, ?, ?, ?, 1)`,
		username, name, passwordHash, string(role),
	)
	if err != nil {
		return User{}, fmt.Errorf("insert user: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return User{}, fmt.Errorf("user id: %w", err)
	}
	return User{ID: id, Username: username, Name: name, PasswordHash: passwordHash, Role: role, Active: true}, nil
}

// UserByUsername looks up a login name.
func (s *Store) UserByUsername(ctx context.Context, username string) (User, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, username, name, password_hash, role, active FROM users WHERE username = ?`,
		strings.TrimSpace(username),
	)
	return scanUser(row)
}

// UserByID loads a user by primary key.
func (s *Store) UserByID(ctx context.Context, id int64) (User, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, username, name, password_hash, role, active FROM users WHERE id = ?`,
		id,
	)
	return scanUser(row)
}

// EnsureAdmin creates the first admin when the table is empty.
// name empty → DefaultName ("Administrator").
func (s *Store) EnsureAdmin(ctx context.Context, username, password, name string) error {
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
	if strings.TrimSpace(name) == "" {
		name = DefaultName
	}
	_, err = s.CreateUser(ctx, username, name, hash, auth.RoleAdmin)
	if err != nil {
		return fmt.Errorf("bootstrap admin: %w", err)
	}
	return nil
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
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
		SELECT u.id, u.username, u.name, u.password_hash, u.role, u.active, s.expires_at
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = ?`, tokenHash,
	).Scan(&u.ID, &u.Username, &u.Name, &u.PasswordHash, &role, &active, &exp)
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
	var keyID int64
	err := s.db.QueryRowContext(ctx, `
		SELECT k.id, u.id, u.username, u.name, u.password_hash, u.role, u.active, k.scope
		FROM api_keys k
		JOIN users u ON u.id = k.user_id
		WHERE k.key_hash = ?`, keyHash,
	).Scan(&keyID, &u.ID, &u.Username, &u.Name, &u.PasswordHash, &role, &active, &scope)
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
	return APIKeyAuth{KeyID: keyID, User: u, Scope: auth.KeyScope(scope)}, nil
}

// TouchAPIKeyLastUsed records a successful API key authentication.
func (s *Store) TouchAPIKeyLastUsed(ctx context.Context, id int64) error {
	if id <= 0 {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `UPDATE api_keys SET last_used_at = datetime('now') WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("touch api key: %w", err)
	}
	return nil
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

// ListUsers returns all users ordered by username.
func (s *Store) ListUsers(ctx context.Context) ([]UserRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, username, name, password_hash, role, active, created_at
		FROM users ORDER BY username COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()
	var out []UserRecord
	for rows.Next() {
		rec, err := scanUserRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

// CountActiveAdmins returns active users with role admin.
func (s *Store) CountActiveAdmins(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM users WHERE role = 'admin' AND active = 1`,
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count admins: %w", err)
	}
	return n, nil
}

// UpdateUser sets role and active for one user.
func (s *Store) UpdateUser(ctx context.Context, id int64, role auth.Role, active bool) error {
	if !auth.ValidRole(role) {
		return fmt.Errorf("invalid role")
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE users SET role = ?, active = ? WHERE id = ?`,
		string(role), boolToInt(active), id,
	)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update user rows: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// RemoveUser deletes a user and their sessions/keys. Archives keep the files;
// uploaded_by is cleared. Refuses the last active admin.
func (s *Store) RemoveUser(ctx context.Context, id int64) error {
	u, err := s.UserByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.GuardLastAdmin(ctx, id, u.Role, false); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin remove user: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, id); err != nil {
		return fmt.Errorf("delete sessions: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM api_keys WHERE user_id = ?`, id); err != nil {
		return fmt.Errorf("delete api keys: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE archives SET uploaded_by = NULL WHERE uploaded_by = ?`, id); err != nil {
		return fmt.Errorf("clear archive actor: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE transit SET uploaded_by = NULL WHERE uploaded_by = ?`, id); err != nil {
		return fmt.Errorf("clear transit actor: %w", err)
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete user rows: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit remove user: %w", err)
	}
	return nil
}

// SetPassword replaces the bcrypt hash for one user and deletes all of their sessions.
func (s *Store) SetPassword(ctx context.Context, id int64, passwordHash string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin set password: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx,
		`UPDATE users SET password_hash = ? WHERE id = ?`,
		passwordHash, id,
	)
	if err != nil {
		return fmt.Errorf("set password: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("set password rows: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, id); err != nil {
		return fmt.Errorf("delete sessions on password change: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit set password: %w", err)
	}
	return nil
}

// SetName replaces the display name for one user.
func (s *Store) SetName(ctx context.Context, id int64, name string) error {
	name, err := NormalizeName(name)
	if err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, `UPDATE users SET name = ? WHERE id = ?`, name, id)
	if err != nil {
		return fmt.Errorf("set name: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("set name rows: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetUsername replaces the login id. Must be unique.
func (s *Store) SetUsername(ctx context.Context, id int64, username string) error {
	username, err := NormalizeUsername(username)
	if err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, `UPDATE users SET username = ? WHERE id = ?`, username, id)
	if err != nil {
		return fmt.Errorf("set username: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("set username rows: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// GuardLastAdmin returns ErrLastAdmin if the change would leave zero active admins.
func (s *Store) GuardLastAdmin(ctx context.Context, userID int64, newRole auth.Role, newActive bool) error {
	u, err := s.UserByID(ctx, userID)
	if err != nil {
		return err
	}
	if u.Role != auth.RoleAdmin || !u.Active {
		return nil
	}
	if newActive && newRole == auth.RoleAdmin {
		return nil
	}
	n, err := s.CountActiveAdmins(ctx)
	if err != nil {
		return err
	}
	if n <= 1 {
		return ErrLastAdmin
	}
	return nil
}

func scanUser(row *sql.Row) (User, error) {
	var u User
	var role string
	var active int
	err := row.Scan(&u.ID, &u.Username, &u.Name, &u.PasswordHash, &role, &active)
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

func scanUserRecord(sc interface {
	Scan(dest ...any) error
}) (UserRecord, error) {
	var rec UserRecord
	var role string
	var active int
	var created string
	err := sc.Scan(&rec.ID, &rec.Username, &rec.Name, &rec.PasswordHash, &role, &active, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return UserRecord{}, ErrNotFound
	}
	if err != nil {
		return UserRecord{}, fmt.Errorf("scan user: %w", err)
	}
	rec.Role = auth.Role(role)
	rec.Active = active != 0
	if t, err := time.Parse(time.RFC3339, created); err == nil {
		rec.CreatedAt = t
	}
	return rec, nil
}
