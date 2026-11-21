package server

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"time"

	"github.com/hrodrig/groot-share/internal/server/core"
	"github.com/hrodrig/groot-share/internal/store"
)

// Actor is the authenticated principal (re-export for server tests).
type Actor = core.Actor

// ParseFilter re-exports core.ParseFilter for server-package tests.
var ParseFilter = core.ParseFilter

func windowSince(window string, now time.Time) time.Time {
	return core.WindowSince(window, now)
}

func parseSort(r *http.Request) (string, bool) {
	return core.ParseSort(r)
}

func sortArchives(items []store.Archive, field string, asc bool) {
	core.SortArchives(items, field, asc)
}
func (s *Server) requestBaseURL(r *http.Request) string {
	return s.RequestBaseURL(r)
}

const sessionCookie = core.SessionCookie

func shellUserData(ac *Actor) map[string]any {
	return core.ShellUserData(ac)
}

func mergeActorData(data map[string]any, ac *Actor) {
	core.MergeActorData(data, ac)
}

func isUniqueViolation(err error) bool {
	return core.IsUniqueViolation(err)
}

func isBrowserForm(r *http.Request) bool {
	return core.IsBrowserForm(r)
}

func (s *Server) openDownload(ctx context.Context, id string) (io.ReadCloser, store.Archive, error) {
	return s.OpenDownload(ctx, id)
}

func (s *Server) uniqueHTTPKey(ctx context.Context, now time.Time) (id, key string, err error) {
	return s.UniqueHTTPKey(ctx, now)
}

func serveBlob(w http.ResponseWriter, r *http.Request, a store.Archive, rc io.ReadCloser) {
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, a.Key))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if rs, ok := rc.(io.ReadSeeker); ok {
		http.ServeContent(w, r, a.Key, a.CreatedAt, rs)
		return
	}
	if a.Size > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(a.Size, 10))
	}
	_, _ = io.Copy(w, rc)
}

func firstFilePart(mr *multipart.Reader) (io.ReadCloser, string, error) {
	for {
		p, err := mr.NextPart()
		if err != nil {
			return nil, "", err
		}
		if p.FileName() != "" {
			return p, p.FileName(), nil
		}
		_ = p.Close()
	}
}
