package store

import (
	"context"
	"fmt"
	"strings"
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

// AuditFilter narrows audit queries. Empty fields mean "no filter".
type AuditFilter struct {
	Actor  string    // case-insensitive substring match; empty = all
	Action string    // exact match; empty = all
	Since  time.Time // created_at >= Since; zero = all
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

// ListAudit returns newest first (first page only).
func (s *Store) ListAudit(ctx context.Context, limit int) ([]Audit, error) {
	return s.ListAuditPage(ctx, limit, 0)
}

// CountAudit returns total audit rows.
func (s *Store) CountAudit(ctx context.Context) (int, error) {
	return s.CountAuditFiltered(ctx, AuditFilter{})
}

// CountAuditFiltered returns the number of audit rows matching f.
func (s *Store) CountAuditFiltered(ctx context.Context, f AuditFilter) (int, error) {
	where, args := auditWhere(f)
	var n int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit"+where, args...).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count audit: %w", err)
	}
	return n, nil
}

// ListAuditPage returns newest first with limit/offset.
func (s *Store) ListAuditPage(ctx context.Context, limit, offset int) ([]Audit, error) {
	return s.ListAuditFiltered(ctx, AuditFilter{}, limit, offset)
}

// ListAuditFiltered returns newest first, matching f, with limit/offset.
func (s *Store) ListAuditFiltered(ctx context.Context, f AuditFilter, limit, offset int) ([]Audit, error) {
	// limit <= 0 means 50 per page; limit < 0 means "no limit" (used by the
	// export path). offset < 0 is clamped to 0.
	noLimit := limit < 0
	if limit <= 0 && !noLimit {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	where, args := auditWhere(f)
	args = append(args, limit, offset)
	q := `
		SELECT id, actor, actor_id, action, object_id, object_key, remote_ip, created_at
		FROM audit` + where + ` ORDER BY id DESC`
	if !noLimit {
		q += ` LIMIT ? OFFSET ?`
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
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

// auditWhere builds a parameterized WHERE clause from f. Returns "" when no
// filter is set. Each clause guards its own arg so the placeholder positions
// stay stable regardless of which fields are set.
func auditWhere(f AuditFilter) (string, []any) {
	var clauses []string
	var args []any
	if f.Actor != "" {
		clauses = append(clauses, "actor LIKE ? ESCAPE '\\'")
		args = append(args, "%"+escapeLike(f.Actor)+"%")
	}
	if f.Action != "" {
		clauses = append(clauses, "action = ?")
		args = append(args, f.Action)
	}
	if !f.Since.IsZero() {
		clauses = append(clauses, "created_at >= ?")
		args = append(args, f.Since.UTC().Format(time.RFC3339))
	}
	if len(clauses) == 0 {
		return "", nil
	}
	return " WHERE " + joinClauses(clauses), args
}

// escapeLike escapes LIKE wildcards so a filter substring is literal.
func escapeLike(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\', '%', '_':
			out = append(out, '\\', s[i])
		default:
			out = append(out, s[i])
		}
	}
	return string(out)
}

func joinClauses(clauses []string) string {
	return strings.Join(clauses, " AND ")
}
