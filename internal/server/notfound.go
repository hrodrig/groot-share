// Package server: friendly 404 surface for browser clients, JSON for API.
package server

import (
	"net/http"
	"strings"
	"text/template"
)

// notFoundTmpl renders the 404 page reusing the page shell (brand, theme,
// favicon, layout CSS). It intentionally omits the app nav: a 404 is
// reached for unmatched routes which may not have an authenticated actor,
// and the primary CTA ("Back to Captures") is enough to recover.
var notFoundTmpl = template.Must(template.New("notfound").Funcs(pageFuncs).Parse(`<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>gfs — Not found</title>{{.FaviconHead}}{{.ThemeHead}}<style>{{.CSS}}</style></head>
<body>
<a class="skip" href="#main">Skip to content</a>
<header class="appbar">
  <div class="appbar-in">
    <div class="appbar-start">
      <a class="brand" href="/">
        <span class="crate" aria-hidden="true"></span>
        <span class="wordmark">gfs</span>
        {{if .BrandSub}}<span class="brand-sub">{{.BrandSub}}</span>{{end}}
      </a>
    </div>
    <div class="appbar-side">
      {{.ThemeToggle}}
    </div>
  </div>
</header>
<main id="main" class="wrap">
<div class="page-head">
  <div>
    <h1>Not found</h1>
    <p class="sub"><code class="mono">{{.Path}}</code> doesn't exist or has been removed.</p>
  </div>
</div>
<section class="card" aria-labelledby="nf-h">
  <div class="card-head">
    <h2 id="nf-h">Page not found</h2>
    <p class="hint">HTTP 404 — check the URL or head back to Captures.</p>
  </div>
  <div style="padding: 16px 24px 20px;">
    <a class="btn" href="/">Back to Captures</a>
  </div>
</section>
</main>
{{.ThemeToggleScript}}
</body></html>
`))

// handleNotFound renders a 404 response. Browser requests (HTML accept, or
// any path that is not the JSON API surface) get the branded page; API
// requests keep the structured JSON error.
func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	if wantsJSON(r) ||
		strings.HasPrefix(r.URL.Path, "/v1/") ||
		strings.HasPrefix(r.URL.Path, "/s/") {
		writeJSONError(w, http.StatusNotFound, "not_found")
		return
	}
	data := s.pageShell()
	data["Path"] = r.URL.Path
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	_ = notFoundTmpl.Execute(w, data)
}
