package store

import (
	"context"
	"fmt"
	"time"
)

// ProtectedArchiveIDs returns the set of archive ids that retention must not
// delete: every archive with at least one pin, plus every archive with at
// least one still-servable share link (not revoked, not expired, not
// exhausted). The share "active" test mirrors IncrementShareUse's WHERE so
// the retention sweep and the download handler agree on what "active" means.
//
// Pins are inherently all-or-nothing (an archive_pins row has no expiry), so
// any pin protects its archive regardless of age; see issue #45.
func (s *Store) ProtectedArchiveIDs(ctx context.Context, now time.Time) (map[string]struct{}, error) {
	prot := map[string]struct{}{}

	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT archive_id FROM archive_pins`)
	if err != nil {
		return nil, fmt.Errorf("protected pins: %w", err)
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, fmt.Errorf("protected pins scan: %w", err)
		}
		prot[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("protected pins rows: %w", err)
	}
	rows.Close()

	rows, err = s.db.QueryContext(ctx, `
		SELECT DISTINCT archive_id FROM share_links
		 WHERE revoked_at = ''
		   AND (expires_at = '' OR expires_at > ?)
		   AND (max_uses = 0 OR use_count < max_uses)`, formatShareTime(now))
	if err != nil {
		return nil, fmt.Errorf("protected shares: %w", err)
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, fmt.Errorf("protected shares scan: %w", err)
		}
		prot[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("protected shares rows: %w", err)
	}
	rows.Close()

	return prot, nil
}
