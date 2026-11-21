package web

import (
	"html/template"
	"strings"

	"github.com/hrodrig/groot-share/internal/config"
	"github.com/hrodrig/groot-share/internal/server/tmpl"
)

func (srv *Web) PageShell() map[string]any {
	data := tmpl.PageShellData(srv.Version)
	data["BrandSub"] = config.DisplayBrandSub(srv.Cfg.BrandSub)
	data["AppFoot"] = srv.footerHTML()
	if srv.Cfg.LoginSimple {
		data["LoginTitle"] = "Sign in"
		data["GateClass"] = "gate gate-simple"
		data["FaviconHead"] = template.HTML("")
	}
	return data
}

func (srv *Web) footerHTML() template.HTML {
	if strings.TrimSpace(srv.Cfg.Footer) == "-" {
		return ""
	}
	if text := config.DisplayFooter(srv.Cfg.Footer); text != "" {
		return template.HTML(`<footer class="app-foot"><p>` + template.HTMLEscapeString(text) + `</p></footer>`)
	}
	return tmpl.AppFootHTML(srv.Version)
}
