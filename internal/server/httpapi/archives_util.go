package httpapi

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/hrodrig/groot-share/internal/store"
)

func wantsPin(r *http.Request) bool {
	h := strings.TrimSpace(strings.ToLower(r.Header.Get("X-GFS-Pin")))
	switch h {
	case "1", "true", "yes", "on":
		return true
	}
	q := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("pin")))
	switch q {
	case "1", "true", "yes", "on":
		return true
	}
	return false
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

func writeJSONDuplicate(w http.ResponseWriter, existing store.Archive) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusConflict)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error":    "duplicate",
		"existing": archiveJSON(existing),
	})
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

// errNotGzip is returned when a raw (non-multipart) upload body does not
// start with the gzip magic bytes. Callers map it to HTTP 400.
var errNotGzip = errors.New("body is not gzip")

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
	br := bufio.NewReader(r.Body)
	magic, err := br.Peek(2)
	if err != nil {
		// Empty/short body: still not a valid gzip archive.
		return nil, "", errNotGzip
	}
	if magic[0] != 0x1f || magic[1] != 0x8b {
		return nil, "", errNotGzip
	}
	return readCloser{Reader: br, c: r.Body}, key, nil
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
