package core

import (
	"net/http"
	"strings"
	"time"

	"github.com/hrodrig/groot-share/internal/store"
)

// auditFilterFrom reads actor/action/window query params into a store filter.
func AuditFilterFrom(r *http.Request) store.AuditFilter {
	q := r.URL.Query()
	window := strings.TrimSpace(q.Get("window"))
	if !allowedWindows[window] {
		window = ""
	}
	return store.AuditFilter{
		Actor:  strings.TrimSpace(q.Get("actor")),
		Action: strings.TrimSpace(q.Get("action")),
		Since:  WindowSince(window, time.Now().UTC()),
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
func MergeAuditFilterData(data map[string]any, f store.AuditFilter) {
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
func CSVField(s string) string {
	if !strings.ContainsAny(s, ",\"\n\r") {
		return s
	}
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

func AuditJSON(ev store.Audit) map[string]any {
	return map[string]any{
		"actor":      ev.Actor,
		"action":     ev.Action,
		"object_id":  ev.ObjectID,
		"object_key": ev.ObjectKey,
		"remote_ip":  ev.RemoteIP,
		"created_at": ev.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}
