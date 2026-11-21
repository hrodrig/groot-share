package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/hrodrig/groot-share/internal/server/core"
)

func (srv *API) HandleListAudit(w http.ResponseWriter, r *http.Request) {
	perPage := parsePageSize(r)
	page := parsePage(r)
	f := core.AuditFilterFrom(r)
	total, err := srv.Store.CountAuditFiltered(r.Context(), f)
	if err != nil {
		slog.Error("count audit", "error", err)
		core.WriteJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	pv := pageViewFor(total, page, perPage)
	offset := (pv.Page - 1) * perPage
	items, err := srv.Store.ListAuditFiltered(r.Context(), f, perPage, offset)
	if err != nil {
		slog.Error("list audit", "error", err)
		core.WriteJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, ev := range items {
		out = append(out, core.AuditJSON(ev))
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
// JSON, honoring the same actor/action/window filtersrv. Admin only.
