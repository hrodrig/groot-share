package web

import (
	"net/http"

	"github.com/hrodrig/groot-share/internal/server/core"
	"github.com/hrodrig/groot-share/internal/server/tmpl"
)

func (srv *Web) loginFail(w http.ResponseWriter, r *http.Request, asJSON bool, code int, msg string) {
	if asJSON {
		core.WriteJSONError(w, code, msg)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	data := srv.PageShell()
	data["Error"] = loginErrorCopy(msg)
	_ = tmpl.Login.Execute(w, data)
}
