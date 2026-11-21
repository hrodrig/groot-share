package web

import (
	"log/slog"
	"net/http"

	"github.com/hrodrig/groot-share/internal/auth"
	"github.com/hrodrig/groot-share/internal/server/core"
	"github.com/hrodrig/groot-share/internal/server/tmpl"
)

func (srv *Web) HandleActivityGET(w http.ResponseWriter, r *http.Request) {
	ac := core.ActorFrom(r.Context())
	pageSize := parsePageSize(r)
	page := parsePage(r)
	f := core.AuditFilterFrom(r)
	total, err := srv.Store.CountAuditFiltered(r.Context(), f)
	if err != nil {
		slog.Error("count audit", "error", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	pv := pageViewFor(total, page, pageSize)
	offset := (pv.Page - 1) * pageSize
	events, err := srv.Store.ListAuditFiltered(r.Context(), f, pageSize, offset)
	if err != nil {
		slog.Error("list audit", "error", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := srv.PageShell()
	core.MergeActorData(data, ac)
	data["Audit"] = events
	data["Pager"] = pv
	data["Nav"] = "activity"
	data["CanExport"] = ac != nil && ac.User.Role == auth.RoleAdmin
	core.MergeAuditFilterData(data, f)
	_ = tmpl.Activity.Execute(w, data)
}
