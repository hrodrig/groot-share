package store

import (
	"context"
	"fmt"
	"time"
)

// Audit is one housekeeping event. Never store passwords or api_keys.
type Audit struct {
	ID        int64
	Actor     string
	ActorID   int64
	Action    string
	ObjectID  string
	ObjectKey string
	RemoteIP  string
	CreatedAt time.Time
}

// InsertAudit records an action. Callers must not pass secrets in any field.
func (s *Store) InsertAudit(ctx context.Context, ev Audit) error {
	if ev.CreatedAt.IsZero() {
		ev.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO audit (actor, actor_id, action, object_id, object_key, remote_ip, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		ev.Actor, ev.ActorID, ev.Action, ev.ObjectID, ev.ObjectKey, ev.RemoteIP,
		ev.CreatedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert audit: %w", err)
	}
	return nil
}

// ListAudit returns newest first.
func (s *Store) ListAudit(ctx context.Context, limit int) ([]Audit, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, actor, actor_id, action, object_id, object_key, remote_ip, created_at
		FROM audit ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list audit: %w", err)
	}
	defer rows.Close()
	out := []Audit{}
	for rows.Next() {
		var ev Audit
		var created string
		if err := rows.Scan(
			&ev.ID, &ev.Actor, &ev.ActorID, &ev.Action,
			&ev.ObjectID, &ev.ObjectKey, &ev.RemoteIP, &created,
		); err != nil {
			return nil, err
		}
		if t, err := time.Parse(time.RFC3339, created); err == nil {
			ev.CreatedAt = t
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}
