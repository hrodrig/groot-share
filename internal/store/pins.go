package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// PinnedArchive is a snapshot of an archive at the time the user pinned it.
// The list intentionally does not join against `archives`: the row stays
// valid even when the archive is in transit (vps-s3) or was deleted. Future
// render layers can mark stale pins (orphan) if needed; 10-01 just shows them.
type PinnedArchive struct {
	ArchiveID  string
	ArchiveKey string
	Size       int64
	CreatedAt  string
}

// AddPin inserts a pin row idempotently. archive_key and size are snapshotted
// from the passed archive so the list keeps rendering when the archive is no
// longer reachable (in transit, deleted, or never reached the local store).
func (s *Store) AddPin(ctx context.Context, userID int64, a Archive) error {
	if userID <= 0 {
		return errors.New("AddPin: userID required")
	}
	if a.ID == "" {
		return errors.New("AddPin: archive id required")
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO archive_pins (user_id, archive_id, archive_key, archive_size, created_at)
		VALUES (?, ?, ?, ?, datetime('now'))`,
		userID, a.ID, a.Key, a.Size,
	)
	if err != nil {
		return fmt.Errorf("AddPin: %w", err)
	}
	return nil
}

// RemovePin deletes a pin row. The bool reports whether a row was actually
// removed (false when the user had not pinned that archive). The operation
// is idempotent: a missing pin returns (false, nil).
func (s *Store) RemovePin(ctx context.Context, userID int64, archiveID string) (bool, error) {
	if userID <= 0 {
		return false, errors.New("RemovePin: userID required")
	}
	if archiveID == "" {
		return false, errors.New("RemovePin: archiveID required")
	}
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM archive_pins WHERE user_id = ? AND archive_id = ?`,
		userID, archiveID,
	)
	if err != nil {
		return false, fmt.Errorf("RemovePin: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("RemovePin rows: %w", err)
	}
	return n > 0, nil
}

// ListPins returns the user's pins ordered newest-first. limit <= 0 means
// no cap (use a sane upper bound at the call site if the caller wants one).
func (s *Store) ListPins(ctx context.Context, userID int64, limit int) ([]PinnedArchive, error) {
	if userID <= 0 {
		return nil, errors.New("ListPins: userID required")
	}
	q := `SELECT archive_id, archive_key, archive_size, created_at
	      FROM archive_pins
	      WHERE user_id = ?
	      ORDER BY created_at DESC, archive_id ASC`
	args := []any{userID}
	if limit > 0 {
		q += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("ListPins: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]PinnedArchive, 0, 8)
	for rows.Next() {
		var p PinnedArchive
		var size sql.NullInt64
		if err := rows.Scan(&p.ArchiveID, &p.ArchiveKey, &size, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("ListPins scan: %w", err)
		}
		if size.Valid {
			p.Size = size.Int64
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ListPins rows: %w", err)
	}
	return out, nil
}
