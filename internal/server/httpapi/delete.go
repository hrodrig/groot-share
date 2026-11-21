package httpapi

import (
	"net/http"

	"github.com/hrodrig/groot-share/internal/server/core"
)

func (srv *API) HandleDelete(w http.ResponseWriter, r *http.Request) {
	// Routing is explicit: JSON API uses DELETE /v1/archives/{id...}
	// and the HTML form uses POST /v1/archives/delete/{id...}. The
	// fragment suffix sniff is gone — the route itself disambiguates.
	id := core.DeleteID(r)
	a, err := srv.RemoveArchive(r.Context(), id)
	if err != nil {
		if r.Method == http.MethodDelete || core.WantsJSON(r) {
			core.WriteJSONError(w, http.StatusNotFound, "not_found")
			return
		}
		srv.NotFound(w, r)
		return
	}
	srv.RecordAudit(r, "delete", a)
	if r.Method == http.MethodDelete || core.WantsJSON(r) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, "/?notice=deleted", http.StatusSeeOther)
}
