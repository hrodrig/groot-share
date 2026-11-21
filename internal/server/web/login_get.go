package web

import (
	"net/http"

	"github.com/hrodrig/groot-share/internal/server/tmpl"
)

func (srv *Web) HandleLoginGET(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := srv.PageShell()
	data["Error"] = ""
	data["Notice"] = ""
	if r.URL.Query().Get("notice") == "password" {
		data["Notice"] = "Password updated. Sign in again."
	}
	_ = tmpl.Login.Execute(w, data)
}
