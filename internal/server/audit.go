package server

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/hrodrig/groot-share/internal/store"
)

func (s *Server) recordAudit(r *http.Request, action string, a store.Archive) {
	if s.Store == nil {
		return
	}
	ev := store.Audit{Action: action, ObjectID: a.ID, ObjectKey: a.Key, RemoteIP: remoteIP(r.RemoteAddr)}
	if u := actorFrom(r.Context()); u != nil {
		ev.Actor = u.Username
		ev.ActorID = u.ID
	}
	if err := s.Store.InsertAudit(r.Context(), ev); err != nil {
		slog.Error("audit insert", "error", err, "action", action)
	}
}

func (s *Server) handleListAudit(w http.ResponseWriter, r *http.Request) {
	items, err := s.Store.ListAudit(r.Context(), 100)
	if err != nil {
		slog.Error("list audit", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, ev := range items {
		out = append(out, auditJSON(ev))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"items": out})
}

func auditJSON(ev store.Audit) map[string]any {
	return map[string]any{
		"actor":      ev.Actor,
		"action":     ev.Action,
		"object_id":  ev.ObjectID,
		"object_key": ev.ObjectKey,
		"remote_ip":  ev.RemoteIP,
		"created_at": ev.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}
