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
	if ac := actorFrom(r.Context()); ac != nil {
		ev.Actor = ac.User.Username
		ev.ActorID = ac.User.ID
	}
	if err := s.Store.InsertAudit(r.Context(), ev); err != nil {
		slog.Error("audit insert", "error", err, "action", action)
	}
}

func (s *Server) recordUserAudit(r *http.Request, action, objectID, objectKey string) {
	if s.Store == nil {
		return
	}
	ev := store.Audit{Action: action, ObjectID: objectID, ObjectKey: objectKey, RemoteIP: remoteIP(r.RemoteAddr)}
	if ac := actorFrom(r.Context()); ac != nil {
		ev.Actor = ac.User.Username
		ev.ActorID = ac.User.ID
	}
	if err := s.Store.InsertAudit(r.Context(), ev); err != nil {
		slog.Error("audit insert", "error", err, "action", action)
	}
}

func (s *Server) handleActivityGET(w http.ResponseWriter, r *http.Request) {
	ac := actorFrom(r.Context())
	pageSize := parsePageSize(r)
	page := parsePage(r)
	total, err := s.Store.CountAudit(r.Context())
	if err != nil {
		slog.Error("count audit", "error", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	pv := pageViewFor(total, page, pageSize)
	offset := (pv.Page - 1) * pageSize
	events, err := s.Store.ListAuditPage(r.Context(), pageSize, offset)
	if err != nil {
		slog.Error("list audit", "error", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := s.pageShell()
	mergeActorData(data, ac)
	data["Audit"] = events
	data["Pager"] = pv
	data["Nav"] = "activity"
	_ = activityTmpl.Execute(w, data)
}

func (s *Server) handleListAudit(w http.ResponseWriter, r *http.Request) {
	perPage := parsePageSize(r)
	page := parsePage(r)
	total, err := s.Store.CountAudit(r.Context())
	if err != nil {
		slog.Error("count audit", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	pv := pageViewFor(total, page, perPage)
	offset := (pv.Page - 1) * perPage
	items, err := s.Store.ListAuditPage(r.Context(), perPage, offset)
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
	_ = json.NewEncoder(w).Encode(map[string]any{
		"items":       out,
		"page":        pv.Page,
		"per_page":    perPage,
		"total":       pv.Total,
		"total_pages": pv.TotalPages,
	})
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
