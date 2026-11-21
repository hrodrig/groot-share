package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/hrodrig/groot-share/internal/server/core"
)

func (srv *API) HandleListArchives(w http.ResponseWriter, r *http.Request) {
	items, err := srv.ListItems(r.Context())
	if err != nil {
		slog.Error("list archives", "error", err)
		core.WriteJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, a := range items {
		out = append(out, archiveJSON(a))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"items": out})
}

func (srv *API) HandleDownload(w http.ResponseWriter, r *http.Request) {
	id := core.DownloadID(r)
	rc, a, err := srv.OpenDownload(r.Context(), id)
	if err != nil {
		srv.NotFound(w, r)
		return
	}
	defer func() { _ = rc.Close() }()
	srv.RecordAudit(r, "download", a)
	serveBlob(w, r, a, rc)
}
