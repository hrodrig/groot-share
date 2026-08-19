package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// SaveTransit records a staged object whose bucket copy has not succeeded yet.
func (s *Store) SaveTransit(ctx context.Context, st Staged, s3Key, lastErr string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO transit (id, key, s3_key, size, sha256, path, uploaded_by, created_at, last_error)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET last_error = excluded.last_error`,
		st.ID, st.Key, s3Key, st.Size, st.SHA256, st.Path, optionalUserID(st.UploadedBy),
		st.CreatedAt.UTC().Format(time.RFC3339), lastErr,
	)
	if err != nil {
		return fmt.Errorf("save transit: %w", err)
	}
	return nil
}

// ListTransit returns in-flight staging rows (oldest first for retry).
func (s *Store) ListTransit(ctx context.Context) ([]Transit, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, key, s3_key, size, sha256, path, COALESCE(uploaded_by, 0), created_at, last_error
		FROM transit ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("list transit: %w", err)
	}
	defer rows.Close()
	out := []Transit{}
	for rows.Next() {
		tr, err := scanTransit(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, tr)
	}
	return out, rows.Err()
}

// TransitBySHA256 loads a pending staging object by content hash.
func (s *Store) TransitBySHA256(ctx context.Context, sha256 string) (Transit, error) {
	if strings.TrimSpace(sha256) == "" {
		return Transit{}, ErrNotFound
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT id, key, s3_key, size, sha256, path, COALESCE(uploaded_by, 0), created_at, last_error
		FROM transit WHERE sha256 = ?
		ORDER BY created_at DESC LIMIT 1`, sha256)
	tr, err := scanTransit(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Transit{}, ErrNotFound
	}
	if err != nil {
		return Transit{}, fmt.Errorf("get transit by sha256: %w", err)
	}
	return tr, nil
}

// TransitByS3Key loads a pending staging object.
func (s *Store) TransitByS3Key(ctx context.Context, s3Key string) (Transit, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, key, s3_key, size, sha256, path, COALESCE(uploaded_by, 0), created_at, last_error
		FROM transit WHERE s3_key = ?`, s3Key)
	tr, err := scanTransit(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Transit{}, ErrNotFound
	}
	if err != nil {
		return Transit{}, fmt.Errorf("get transit: %w", err)
	}
	return tr, nil
}

// DeleteTransit removes the row and the staging file.
func (s *Store) DeleteTransit(ctx context.Context, id string) error {
	var path string
	err := s.db.QueryRowContext(ctx, `SELECT path FROM transit WHERE id = ?`, id).Scan(&path)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("get transit path: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM transit WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete transit: %w", err)
	}
	_ = os.Remove(path)
	return nil
}

// UpdateTransitError records the last copy failure.
func (s *Store) UpdateTransitError(ctx context.Context, id, lastErr string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE transit SET last_error = ? WHERE id = ?`, lastErr, id)
	if err != nil {
		return fmt.Errorf("update transit: %w", err)
	}
	return nil
}

func scanTransit(sc rowScanner) (Transit, error) {
	var tr Transit
	var created string
	if err := sc.Scan(
		&tr.ID, &tr.Key, &tr.S3Key, &tr.Size, &tr.SHA256, &tr.Path,
		&tr.UploadedBy, &created, &tr.LastError,
	); err != nil {
		return Transit{}, err
	}
	if t, err := time.Parse(time.RFC3339, created); err == nil {
		tr.CreatedAt = t
	}
	return tr, nil
}
