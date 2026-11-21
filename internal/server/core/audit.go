package core

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/hrodrig/groot-share/internal/store"
)

func (c *Core) InsertAudit(ctx context.Context, ev store.Audit) error {
	if c.AuditInsert != nil {
		return c.AuditInsert(ctx, ev)
	}
	if c.Store == nil {
		return fmt.Errorf("no store")
	}
	return c.Store.InsertAudit(ctx, ev)
}

func (c *Core) RecordAudit(r *http.Request, action string, a store.Archive) {
	if c.Store == nil {
		return
	}
	ev := store.Audit{Action: action, ObjectID: a.ID, ObjectKey: a.Key, RemoteIP: RemoteIP(r.RemoteAddr)}
	if ac := ActorFrom(r.Context()); ac != nil {
		ev.Actor = ac.User.Username
		ev.ActorID = ac.User.ID
	}
	if err := c.InsertAudit(r.Context(), ev); err != nil {
		slog.Error("audit insert", "error", err, "action", action)
	}
}

func (c *Core) RecordUserAudit(r *http.Request, action, objectID, objectKey string) {
	if c.Store == nil {
		return
	}
	ev := store.Audit{Action: action, ObjectID: objectID, ObjectKey: objectKey, RemoteIP: RemoteIP(r.RemoteAddr)}
	if ac := ActorFrom(r.Context()); ac != nil {
		ev.Actor = ac.User.Username
		ev.ActorID = ac.User.ID
	}
	if err := c.InsertAudit(r.Context(), ev); err != nil {
		slog.Error("audit insert", "error", err, "action", action)
	}
}

func (c *Core) RequireUserAudit(r *http.Request, action, objectID, objectKey string) error {
	if c.Store == nil {
		return fmt.Errorf("audit unavailable")
	}
	ev := store.Audit{Action: action, ObjectID: objectID, ObjectKey: objectKey, RemoteIP: RemoteIP(r.RemoteAddr)}
	if ac := ActorFrom(r.Context()); ac != nil {
		ev.Actor = ac.User.Username
		ev.ActorID = ac.User.ID
	}
	if err := c.InsertAudit(r.Context(), ev); err != nil {
		slog.Error("audit insert required", "error", err, "action", action)
		return fmt.Errorf("audit unavailable")
	}
	return nil
}
