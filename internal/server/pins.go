package server

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/hrodrig/groot-share/internal/auth"
	"github.com/hrodrig/groot-share/internal/store"
)

// pinID parses the archive id from a /v1/pin/archives/{id...} path. The
// leading "/v1/pin/archives/" prefix is stripped by the mux before this
// runs, so PathValue("id") already holds the archive id (possibly with a
// trailing "/delete" from the form-alias path).
func pinID(r *http.Request) string {
	id := strings.Trim(r.PathValue("id"), "/")
	id = strings.TrimSuffix(id, "/delete")
	return strings.Trim(id, "/")
}

// pinPathIsDelete reports whether the unpin form-action path was used (i.e.
// the original URL ended in /delete). The mux strips the prefix, so the
// remaining PathValue may carry a "/delete" suffix we have to inspect.
func pinPathIsDelete(r *http.Request) bool {
	return strings.HasSuffix(strings.Trim(r.PathValue("id"), "/"), "/delete")
}

// handlePin toggles a pin row for the calling user. POST creates, DELETE
// removes. The unpin form-action posts to the same handler with a trailing
// /delete (mirroring how deleteID handles .../delete on archives); that
// collapses to the DELETE branch even though the HTTP method is POST.
func (s *Server) handlePin(w http.ResponseWriter, r *http.Request) {
	ac := actorFrom(r.Context())
	if ac == nil || ac.User.ID == 0 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := pinID(r)
	if id == "" {
		http.Error(w, "bad_request", http.StatusBadRequest)
		return
	}
	isDelete := r.Method == http.MethodDelete || pinPathIsDelete(r)
	if isDelete {
		if _, err := s.Store.RemovePin(r.Context(), ac.User.ID, id); err != nil {
			slog.Error("pin remove", "id", id, "user", ac.User.ID, "error", err)
			http.Error(w, "internal", http.StatusInternalServerError)
			return
		}
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		// Form post: redirect back to Captures.
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method_not_allowed", http.StatusMethodNotAllowed)
		return
	}
	a, err := s.resolveArchiveForPin(r, id)
	if err != nil {
		if err == store.ErrNotFound {
			if wantsJSON(r) {
				writeJSONError(w, http.StatusNotFound, "not_found")
				return
			}
			http.Redirect(w, r, "/?notice=pin_not_found", http.StatusSeeOther)
			return
		}
		slog.Error("pin resolve", "id", id, "error", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	if err := s.Store.AddPin(r.Context(), ac.User.ID, a); err != nil {
		slog.Error("pin add", "id", id, "user", ac.User.ID, "error", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	if wantsJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"pinned":true}`))
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// resolveArchiveForPin returns the archive metadata the pin row should
// snapshot. On VPS + S3 the id may be a bucket key, so we look it up via the
// same path the download uses. We do NOT silently accept unknown ids — the
// row has to point at a real archive so the UI link is clickable.
func (s *Server) resolveArchiveForPin(r *http.Request, id string) (store.Archive, error) {
	_, a, err := s.openDownload(r.Context(), id)
	if err != nil {
		return store.Archive{}, err
	}
	return a, nil
}

// pinRoutes registers the pin endpoints. We use PermArchivesRead (granted to
// viewer, uploader, and admin) because pinning is a personal UI preference,
// not a privileged action. The unpin form-action posts to the same path
// with a trailing /delete; the handler collapses both DELETE and POST-with-
// /delete to the same RemovePin branch.
func (s *Server) pinRoutes(mux *http.ServeMux) {
	mux.Handle("POST /v1/pin/archives/{id...}", s.requirePermission(auth.PermArchivesRead, s.handlePin))
	mux.Handle("DELETE /v1/pin/archives/{id...}", s.requirePermission(auth.PermArchivesRead, s.handlePin))
}
