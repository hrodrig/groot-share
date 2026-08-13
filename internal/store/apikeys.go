package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/hrodrig/groot-share/internal/auth"
)

// APIKeyRecord is a stored api_key row (never includes the secret).
type APIKeyRecord struct {
	ID        int64
	UserID    int64
	Prefix    string
	Scope     auth.KeyScope
	CreatedAt time.Time
}

// ListAPIKeysByUser returns keys for one user, newest first.
func (s *Store) ListAPIKeysByUser(ctx context.Context, userID int64) ([]APIKeyRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, prefix, scope, created_at
		FROM api_keys WHERE user_id = ?
		ORDER BY id DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	defer rows.Close()
	var out []APIKeyRecord
	for rows.Next() {
		rec, err := scanAPIKeyRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

// APIKeyByID loads one key row.
func (s *Store) APIKeyByID(ctx context.Context, id int64) (APIKeyRecord, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, prefix, scope, created_at
		FROM api_keys WHERE id = ?`, id)
	return scanAPIKeyRecordRow(row)
}

// DeleteAPIKey removes a key owned by userID.
func (s *Store) DeleteAPIKey(ctx context.Context, id, userID int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM api_keys WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return fmt.Errorf("delete api key: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete api key rows: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func scanAPIKeyRecord(sc interface {
	Scan(dest ...any) error
}) (APIKeyRecord, error) {
	var rec APIKeyRecord
	var scope string
	var created string
	err := sc.Scan(&rec.ID, &rec.UserID, &rec.Prefix, &scope, &created)
	if err != nil {
		return APIKeyRecord{}, err
	}
	rec.Scope = auth.KeyScope(scope)
	if t, err := time.Parse(time.RFC3339, created); err == nil {
		rec.CreatedAt = t
	}
	return rec, nil
}

func scanAPIKeyRecordRow(row *sql.Row) (APIKeyRecord, error) {
	rec, err := scanAPIKeyRecord(row)
	if errors.Is(err, sql.ErrNoRows) {
		return APIKeyRecord{}, ErrNotFound
	}
	if err != nil {
		return APIKeyRecord{}, fmt.Errorf("get api key: %w", err)
	}
	return rec, nil
}
