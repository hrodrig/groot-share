package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/hrodrig/groot-share/internal/server/core"
	"github.com/hrodrig/groot-share/internal/store"
)

func (srv *API) HandleUpload(w http.ResponseWriter, r *http.Request) {
	ac := core.ActorFrom(r.Context())
	r.Body = http.MaxBytesReader(w, r.Body, srv.MaxUpload())
	src, key, err := uploadReader(r)
	if err != nil {
		status := http.StatusBadRequest
		notice := "upload_error"
		if isMaxBytes(err) {
			status = http.StatusRequestEntityTooLarge
			notice = "too_large"
		}
		if core.IsBrowserForm(r) {
			http.Redirect(w, r, "/upload?notice="+notice, http.StatusSeeOther)
			return
		}
		core.WriteJSONError(w, status, "bad_request")
		return
	}
	defer func() { _ = src.Close() }()
	a, err := srv.IngestBody(r.Context(), src, key, ac.User.ID)
	if err != nil {
		var dup *store.DuplicateError
		if errors.As(err, &dup) {
			if core.IsBrowserForm(r) {
				http.Redirect(w, r, "/upload?notice=duplicate&name="+url.QueryEscape(store.SanitizeArchiveKey(key)), http.StatusSeeOther)
				return
			}
			writeJSONDuplicate(w, dup.Existing)
			return
		}
		if core.IsBrowserForm(r) {
			notice := "upload_error"
			if isMaxBytes(err) {
				notice = "too_large"
			}
			http.Redirect(w, r, "/upload?notice="+notice, http.StatusSeeOther)
			return
		}
		if isMaxBytes(err) {
			core.WriteJSONError(w, http.StatusRequestEntityTooLarge, "too_large")
			return
		}
		slog.Error("ingest failed", "error", err)
		core.WriteJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	srv.RecordAudit(r, "upload", a)
	if wantsPin(r) && ac != nil {
		// Fail-open: archive is already stored; pin failure must not fail the upload.
		if err := srv.Store.AddPin(r.Context(), ac.User.ID, a); err != nil {
			slog.Error("pin on ingest", "error", err, "archive_id", a.ID)
		}
	}
	if core.IsBrowserForm(r) {
		http.Redirect(w, r, "/upload?notice=uploaded&name="+url.QueryEscape(a.Key), http.StatusSeeOther)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(archiveJSON(a))
}
