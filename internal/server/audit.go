package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/hrodrig/groot-share/internal/auth"
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
	f := auditFilterFrom(r)
	total, err := s.Store.CountAuditFiltered(r.Context(), f)
	if err != nil {
		slog.Error("count audit", "error", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	pv := pageViewFor(total, page, pageSize)
	offset := (pv.Page - 1) * pageSize
	events, err := s.Store.ListAuditFiltered(r.Context(), f, pageSize, offset)
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
	data["CanExport"] = ac != nil && ac.User.Role == auth.RoleAdmin
	mergeAuditFilterData(data, f)
	_ = activityTmpl.Execute(w, data)
}

func (s *Server) handleListAudit(w http.ResponseWriter, r *http.Request) {
	perPage := parsePageSize(r)
	page := parsePage(r)
	f := auditFilterFrom(r)
	total, err := s.Store.CountAuditFiltered(r.Context(), f)
	if err != nil {
		slog.Error("count audit", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	pv := pageViewFor(total, page, perPage)
	offset := (pv.Page - 1) * perPage
	items, err := s.Store.ListAuditFiltered(r.Context(), f, perPage, offset)
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

// handleActivityExport streams the full (un-paginated) activity log as CSV or
// JSON, honoring the same actor/action/window filters. Admin only.
func (s *Server) handleActivityExport(w http.ResponseWriter, r *http.Request) {
	ac := actorFrom(r.Context())
	if ac == nil || ac.User.Role != auth.RoleAdmin {
		writeJSONError(w, http.StatusForbidden, "admin required")
		return
	}
	f := auditFilterFrom(r)
	items, err := s.Store.ListAuditFiltered(r.Context(), f, -1, 0)
	if err != nil {
		slog.Error("export audit", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	format := strings.TrimSpace(r.URL.Query().Get("format"))
	if format != "csv" {
		format = "json"
	}
	if format == "json" {
		out := make([]map[string]any, 0, len(items))
		for _, ev := range items {
			out = append(out, auditJSON(ev))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", `attachment; filename="activity.json"`)
		_ = json.NewEncoder(w).Encode(map[string]any{"items": out, "total": len(out)})
		return
	}
	var b strings.Builder
	b.WriteString("actor,action,object_id,object_key,remote_ip,created_at\n")
	for _, ev := range items {
		b.WriteString(csvField(ev.Actor))
		b.WriteByte(',')
		b.WriteString(csvField(ev.Action))
		b.WriteByte(',')
		b.WriteString(csvField(ev.ObjectID))
		b.WriteByte(',')
		b.WriteString(csvField(ev.ObjectKey))
		b.WriteByte(',')
		b.WriteString(csvField(ev.RemoteIP))
		b.WriteByte(',')
		b.WriteString(csvField(ev.CreatedAt.UTC().Format(time.RFC3339)))
		b.WriteByte('\n')
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="activity.csv"`)
	_, _ = w.Write([]byte(b.String()))
}

// auditFilterFrom reads actor/action/window query params into a store filter.
func auditFilterFrom(r *http.Request) store.AuditFilter {
	q := r.URL.Query()
	window := strings.TrimSpace(q.Get("window"))
	if !allowedWindows[window] {
		window = ""
	}
	return store.AuditFilter{
		Actor:  strings.TrimSpace(q.Get("actor")),
		Action: strings.TrimSpace(q.Get("action")),
		Since:  windowSince(window, time.Now().UTC()),
	}
}

// auditActions is the fixed catalog of action values offered in the filter
// dropdown. New actions are added here when auditing expands.
var auditActions = []string{
	"upload", "download", "delete",
	"user.create", "user.update", "user.deactivate",
	"apikey.revoke", "share_create", "share_revoke",
}

// mergeAuditFilterData puts the filter state into the template data so the
// filter form re-renders the current selection.
func mergeAuditFilterData(data map[string]any, f store.AuditFilter) {
	data["FilterActor"] = f.Actor
	data["FilterAction"] = f.Action
	data["FilterWindow"] = func() string {
		if f.Since.IsZero() {
			return ""
		}
		// Recover the window token for the radio/chip selection.
		d := time.Since(f.Since).Round(time.Hour)
		switch {
		case d >= 29*24*time.Hour:
			return "30d"
		case d >= 6*24*time.Hour:
			return "7d"
		default:
			return "24h"
		}
	}()
	data["AuditActions"] = auditActions
}

// csvField quotes a CSV field if it contains a comma, quote, or newline.
func csvField(s string) string {
	if !strings.ContainsAny(s, ",\"\n\r") {
		return s
	}
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
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
