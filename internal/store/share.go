package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ShareLink is a time-limited external download link to one archive.
// TokenHash is the SHA-256 of the raw token; the raw token is never stored.
type ShareLink struct {
	ID        int64
	ArchiveID string
	TokenHash string
	CreatedBy int64
	Label     string
	MaxUses   int // 0 = unlimited
	UseCount  int
	ExpiresAt time.Time // zero = never
	RevokedAt time.Time // zero = active
	CreatedAt time.Time
}

// Active reports whether the link can still serve a download.
func (l ShareLink) Active(now time.Time) bool {
	if !l.RevokedAt.IsZero() {
		return false
	}
	if !l.ExpiresAt.IsZero() && !now.Before(l.ExpiresAt) {
		return false
	}
	if l.MaxUses > 0 && l.UseCount >= l.MaxUses {
		return false
	}
	return true
}

var errShareNotFound = errors.New("share link not found")

func shareTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func formatShareTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// CreateShareLink inserts a share link and returns it. tokenHash is the SHA-256
// of the raw token; the raw token is shown once by the caller and never stored.
func (s *Store) CreateShareLink(ctx context.Context, archiveID, tokenHash string, createdBy int64, label string, maxUses int, expiresAt time.Time) (ShareLink, error) {
	if expiresAt.IsZero() {
		expiresAt = time.Now().UTC().Add(24 * time.Hour)
	}
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO share_links (archive_id, token_hash, created_by, label, max_uses, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		archiveID, tokenHash, createdBy, label, maxUses, formatShareTime(expiresAt), formatShareTime(time.Now().UTC()),
	)
	if err != nil {
		return ShareLink{}, fmt.Errorf("insert share link: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return ShareLink{}, fmt.Errorf("share link id: %w", err)
	}
	return ShareLink{
		ID:        id,
		ArchiveID: archiveID,
		TokenHash: tokenHash,
		CreatedBy: createdBy,
		Label:     label,
		MaxUses:   maxUses,
		UseCount:  0,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now().UTC(),
	}, nil
}

// ShareByTokenHash loads an active-or-expired link by its hashed token. It
// returns errShareNotFound when the token is unknown (callers map to 404).
func (s *Store) ShareByTokenHash(ctx context.Context, tokenHash string) (ShareLink, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, archive_id, token_hash, created_by, label, max_uses, use_count, expires_at, revoked_at, created_at
		FROM share_links WHERE token_hash = ?`, tokenHash)
	var l ShareLink
	var expires, revoked, created string
	if err := row.Scan(
		&l.ID, &l.ArchiveID, &l.TokenHash, &l.CreatedBy, &l.Label,
		&l.MaxUses, &l.UseCount, &expires, &revoked, &created,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ShareLink{}, errShareNotFound
		}
		return ShareLink{}, fmt.Errorf("share by token: %w", err)
	}
	l.ExpiresAt = shareTime(expires)
	l.RevokedAt = shareTime(revoked)
	l.CreatedAt = shareTime(created)
	return l, nil
}

// ListShareLinks returns links for an archive, newest first.
func (s *Store) ListShareLinks(ctx context.Context, archiveID string) ([]ShareLink, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, archive_id, token_hash, created_by, label, max_uses, use_count, expires_at, revoked_at, created_at
		FROM share_links WHERE archive_id = ? ORDER BY id DESC`, archiveID)
	if err != nil {
		return nil, fmt.Errorf("list share links: %w", err)
	}
	defer rows.Close()
	out := []ShareLink{}
	for rows.Next() {
		var l ShareLink
		var expires, revoked, created string
		if err := rows.Scan(
			&l.ID, &l.ArchiveID, &l.TokenHash, &l.CreatedBy, &l.Label,
			&l.MaxUses, &l.UseCount, &expires, &revoked, &created,
		); err != nil {
			return nil, err
		}
		l.ExpiresAt = shareTime(expires)
		l.RevokedAt = shareTime(revoked)
		l.CreatedAt = shareTime(created)
		out = append(out, l)
	}
	return out, rows.Err()
}

// RevokeShareLink sets revoked_at on a link owned by an admin's archive.
// It returns nil on success; ErrNotFound is not wrapped so handlers 404.
func (s *Store) RevokeShareLink(ctx context.Context, shareID int64, now time.Time) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE share_links SET revoked_at = ? WHERE id = ? AND revoked_at = ''`,
		formatShareTime(now), shareID,
	)
	if err != nil {
		return fmt.Errorf("revoke share link: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("revoke share link rows: %w", err)
	}
	if n == 0 {
		return errShareNotFound
	}
	return nil
}

// IncrementShareUse bumps use_count atomically and returns the new count plus
// whether the link is still usable (active, not exhausted). Callers check
// Active() before incrementing to avoid race on the last use.
func (s *Store) IncrementShareUse(ctx context.Context, id int64) (int, error) {
	res, err := s.db.ExecContext(ctx, `
		UPDATE share_links SET use_count = use_count + 1 WHERE id = ?`, id)
	if err != nil {
		return 0, fmt.Errorf("increment share use: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("increment share use rows: %w", err)
	}
	if n == 0 {
		return 0, errShareNotFound
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT use_count FROM share_links WHERE id = ?`, id).Scan(&count); err != nil {
		return 0, fmt.Errorf("read share use: %w", err)
	}
	return count, nil
}
