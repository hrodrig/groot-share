package server

import (
	"html/template"
	"strings"

	"github.com/hrodrig/groot-share/internal/config"
	"github.com/hrodrig/groot-share/internal/server/tmpl"
)

func (s *Server) pageShell() map[string]any {
	data := tmpl.PageShellData(s.Version)
	data["BrandSub"] = config.DisplayBrandSub(s.Cfg.BrandSub)
	data["AppFoot"] = s.footerHTML()
	if s.Cfg.LoginSimple {
		data["LoginTitle"] = "Sign in"
		data["GateClass"] = "gate gate-simple"
		data["FaviconHead"] = template.HTML("")
	}
	return data
}

func (s *Server) footerHTML() template.HTML {
	if strings.TrimSpace(s.Cfg.Footer) == "-" {
		return ""
	}
	if text := config.DisplayFooter(s.Cfg.Footer); text != "" {
		return template.HTML(`<footer class="app-foot"><p>` + template.HTMLEscapeString(text) + `</p></footer>`)
	}
	return tmpl.AppFootHTML(s.Version)
}
