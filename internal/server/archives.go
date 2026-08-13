package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/hrodrig/groot-share/internal/store"
)

func (s *Server) maxUpload() int64 {
	if s.Cfg.MaxUploadBytes > 0 {
		return s.Cfg.MaxUploadBytes
	}
	return 32 << 30
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	u := actorFrom(r.Context())
	if u == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	items, err := s.listItems(r.Context())
	if err != nil {
		slog.Error("list archives", "error", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = homeTmpl.Execute(w, map[string]any{
		"CSS":      template.CSS(layoutCSS),
		"Username": u.Username,
		"Admin":    u.Admin,
		"Items":    items,
	})
}

func (s *Server) handleListArchives(w http.ResponseWriter, r *http.Request) {
	items, err := s.listItems(r.Context())
	if err != nil {
		slog.Error("list archives", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, a := range items {
		out = append(out, archiveJSON(a))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"items": out})
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	u := actorFrom(r.Context())
	if u == nil {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, s.maxUpload())
	src, key, err := uploadReader(r)
	if err != nil {
		status := http.StatusBadRequest
		if isMaxBytes(err) {
			status = http.StatusRequestEntityTooLarge
		}
		if wantsJSON(r) || strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			writeJSONError(w, status, "bad_request")
			return
		}
		http.Error(w, "bad request", status)
		return
	}
	defer func() { _ = src.Close() }()
	a, err := s.ingestBody(r.Context(), src, key, u.ID)
	if err != nil {
		if isMaxBytes(err) {
			writeJSONError(w, http.StatusRequestEntityTooLarge, "too_large")
			return
		}
		slog.Error("ingest failed", "error", err)
		writeJSONError(w, http.StatusBadRequest, "bad_request")
		return
	}
	if !wantsJSON(r) && strings.Contains(r.Header.Get("Content-Type"), "multipart/") {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(archiveJSON(a))
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	id := downloadID(r)
	rc, a, err := s.openDownload(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer func() { _ = rc.Close() }()
	serveBlob(w, r, a, rc)
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
		w.Header().Set("Content-Length", fmt.Sprintf("%d", a.Size))
	}
	_, _ = io.Copy(w, rc)
}

func archiveJSON(a store.Archive) map[string]any {
	out := map[string]any{
		"id":             a.ID,
		"key":            a.Key,
		"size":           a.Size,
		"etag_or_sha256": a.SHA256,
		"created_at":     a.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		"source":         a.Source,
	}
	if a.Storage != "" {
		out["storage"] = a.Storage
	}
	return out
}

type readCloser struct {
	io.Reader
	c io.Closer
}

func (r readCloser) Close() error {
	if r.c != nil {
		return r.c.Close()
	}
	return nil
}

func uploadReader(r *http.Request) (io.ReadCloser, string, error) {
	ct := r.Header.Get("Content-Type")
	media, _, _ := mime.ParseMediaType(ct)
	if strings.HasPrefix(media, "multipart/") {
		mr, err := r.MultipartReader()
		if err != nil {
			return nil, "", err
		}
		return firstFilePart(mr)
	}
	key := r.Header.Get("X-Gfs-Filename")
	if key == "" {
		key = "archive.tar.gz"
	}
	return readCloser{Reader: r.Body, c: r.Body}, key, nil
}

func firstFilePart(mr *multipart.Reader) (io.ReadCloser, string, error) {
	for {
		p, err := mr.NextPart()
		if err != nil {
			return nil, "", err
		}
		name := p.FormName()
		if name == "file" || p.FileName() != "" {
			key := p.FileName()
			if key == "" {
				key = "archive.tar.gz"
			}
			return p, key, nil
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(p, 1<<20))
		_ = p.Close()
	}
}

func isMaxBytes(err error) bool {
	var maxErr *http.MaxBytesError
	return errors.As(err, &maxErr)
}
