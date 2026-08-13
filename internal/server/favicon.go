package server

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/hrodrig/groot-share/assets"
)

var faviconAssetNames = []string{
	"favicon.ico",
	"favicon.svg",
	"favicon-16x16.png",
	"favicon-32x32.png",
	"apple-touch-icon.png",
	"android-chrome-192x192.png",
	"android-chrome-512x512.png",
}

func mountFaviconRoutes(mux *http.ServeMux) {
	favFS, err := fs.Sub(assets.FaviconsFS, "favicons")
	if err != nil {
		panic("assets/favicons: " + err.Error())
	}
	const cache = "no-cache, must-revalidate"
	for _, name := range faviconAssetNames {
		n := name
		mux.Handle("GET /static/"+n, withCacheControl(cache, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			setStaticContentType(w, n)
			http.ServeFileFS(w, r, favFS, n)
		})))
	}
	mux.Handle("GET /static/manifest.json", withCacheControl(cache, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/manifest+json; charset=utf-8")
		http.ServeFileFS(w, r, favFS, "manifest.json")
	})))
	mux.Handle("GET /favicon.ico", http.RedirectHandler("/static/favicon.ico", http.StatusMovedPermanently))
}

func setStaticContentType(w http.ResponseWriter, name string) {
	switch {
	case strings.HasSuffix(name, ".svg"):
		w.Header().Set("Content-Type", "image/svg+xml")
	case strings.HasSuffix(name, ".png"):
		w.Header().Set("Content-Type", "image/png")
	case strings.HasSuffix(name, ".ico"):
		w.Header().Set("Content-Type", "image/x-icon")
	}
}

func withCacheControl(value string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", value)
		next.ServeHTTP(w, r)
	})
}
