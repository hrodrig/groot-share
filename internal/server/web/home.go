package web

import (
	"log/slog"
	"net/http"

	"github.com/hrodrig/groot-share/internal/server/core"
	"github.com/hrodrig/groot-share/internal/server/tmpl"
	"github.com/hrodrig/groot-share/internal/store"
)

func (srv *Web) HandleHome(w http.ResponseWriter, r *http.Request) {
	ac := core.ActorFrom(r.Context())
	// Full inventory: cluster chip counts come from this. (D-06: chips
	// reflect the unfiltered list, not the post-filter one.)
	allItems, err := srv.ListItems(r.Context())
	if err != nil {
		slog.Error("list archives", "error", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	var total int64
	var incomplete int
	for i := range allItems {
		if allItems[i].Storage == "" {
			allItems[i].Storage = "local"
		}
		total += allItems[i].Size
		if allItems[i].Storage == "transit" {
			incomplete++
		}
	}
	// Apply the request's filter on top of the full list.
	filter := core.ParseFilter(r)
	items := core.ApplyFilterInMemory(allItems, filter)
	topo := "vps"
	if srv.UseBucket() {
		topo = "vps-s3"
	}
	pageSize := parsePageSize(r)
	sortField, sortAsc := core.ParseSort(r)
	core.SortArchives(items, sortField, sortAsc)
	page := parsePage(r)
	pageItems, pager := paginateSlice(items, page, pageSize)
	applySortQuery(&pager, sortField, sortAsc)
	noticeKind, noticeText := noticeFromQuery(r.URL.Query())
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := srv.PageShell()
	core.MergeActorData(data, ac)
	data["Items"] = pageItems
	completeness := make(map[string]*core.CompletenessBadge)
	for i := range pageItems {
		if b, ok := srv.CompletenessBadgeOf(pageItems[i]); ok {
			completeness[pageItems[i].ID] = &b
		}
	}
	data["Completeness"] = completeness
	data["StatsLine"] = statsLine(len(items), total)
	data["Summary"] = map[string]any{
		"Count":           len(allItems),
		"Bytes":           total,
		"ClusterCount":    len(store.ClusterCounts(allItems)),
		"IncompleteCount": incomplete,
		"StorageTopology": topo,
	}
	data["Filter"] = filter
	data["ClusterChips"] = store.ClusterCounts(allItems)
	data["FilterURL"] = FilterURLBuilder{Base: r.URL.RawQuery}
	data["MaxUpload"] = srv.MaxUpload()
	data["Pager"] = pager
	data["NoticeKind"] = noticeKind
	data["NoticeText"] = noticeText
	data["Nav"] = "captures"
	data["BaseURL"] = srv.RequestBaseURL(r)
	if ac != nil && ac.User.ID != 0 {
		pins, perr := srv.Store.ListPins(r.Context(), ac.User.ID, 16)
		if perr != nil {
			slog.Warn("list pins", "user", ac.User.ID, "error", perr)
		} else {
			data["Pins"] = pins
		}
	}
	_ = tmpl.Home.Execute(w, data)
}
