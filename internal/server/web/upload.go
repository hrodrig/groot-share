package web

import (
	"net/http"

	"github.com/hrodrig/groot-share/internal/server/core"
	"github.com/hrodrig/groot-share/internal/server/tmpl"
)

func (srv *Web) HandleUploadGET(w http.ResponseWriter, r *http.Request) {
	ac := core.ActorFrom(r.Context())
	noticeKind, noticeText := noticeFromQuery(r.URL.Query())
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := srv.PageShell()
	core.MergeActorData(data, ac)
	data["MaxUpload"] = srv.MaxUpload()
	data["NoticeKind"] = noticeKind
	data["NoticeText"] = noticeText
	data["Nav"] = "upload"
	_ = tmpl.Upload.Execute(w, data)
}
