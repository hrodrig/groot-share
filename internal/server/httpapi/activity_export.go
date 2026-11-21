package httpapi

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/hrodrig/groot-share/internal/auth"
	"github.com/hrodrig/groot-share/internal/server/core"
)

func (srv *API) HandleActivityExport(w http.ResponseWriter, r *http.Request) {
	ac := core.ActorFrom(r.Context())
	if ac == nil || ac.User.Role != auth.RoleAdmin {
		core.WriteJSONError(w, http.StatusForbidden, "admin required")
		return
	}
	f := core.AuditFilterFrom(r)
	items, err := srv.Store.ListAuditFiltered(r.Context(), f, -1, 0)
	if err != nil {
		slog.Error("export audit", "error", err)
		core.WriteJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	format := strings.TrimSpace(r.URL.Query().Get("format"))
	if format != "csv" {
		format = "json"
	}
	if format == "json" {
		out := make([]map[string]any, 0, len(items))
		for _, ev := range items {
			out = append(out, core.AuditJSON(ev))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="gfs-activity-%s.json"`, time.Now().UTC().Format("200601021504")))
		_ = json.NewEncoder(w).Encode(map[string]any{"items": out, "total": len(out)})
		return
	}
	var b strings.Builder
	b.WriteString("actor,action,object_id,object_key,remote_ip,created_at\n")
	for _, ev := range items {
		b.WriteString(core.CSVField(ev.Actor))
		b.WriteByte(',')
		b.WriteString(core.CSVField(ev.Action))
		b.WriteByte(',')
		b.WriteString(core.CSVField(ev.ObjectID))
		b.WriteByte(',')
		b.WriteString(core.CSVField(ev.ObjectKey))
		b.WriteByte(',')
		b.WriteString(core.CSVField(ev.RemoteIP))
		b.WriteByte(',')
		b.WriteString(core.CSVField(ev.CreatedAt.UTC().Format(time.RFC3339)))
		b.WriteByte('\n')
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="gfs-activity-%s.csv"`, time.Now().UTC().Format("200601021504")))
	_, _ = w.Write([]byte(b.String()))
}
