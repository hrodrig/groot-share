package server

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
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
	ac := actorFrom(r.Context())
	// Full inventory: cluster chip counts come from this. (D-06: chips
	// reflect the unfiltered list, not the post-filter one.)
	allItems, err := s.listItems(r.Context())
	if err != nil {
		slog.Error("list archives", "error", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	var total int64
	var incomplete int
	for i := range allItems {
		if allItems[i].Storage == "" {
			allItems[i].Storage = "local"
		}
		total += allItems[i].Size
		if allItems[i].Storage == "transit" {
			incomplete++
		}
	}
	// Apply the request's filter on top of the full list.
	filter := ParseFilter(r)
	items := applyFilterInMemory(allItems, filter)
	topo := "vps"
	if s.useBucket() {
		topo = "vps-s3"
	}
	pageSize := parsePageSize(r)
	sortField, sortAsc := parseSort(r)
	sortArchives(items, sortField, sortAsc)
	page := parsePage(r)
	pageItems, pager := paginateSlice(items, page, pageSize)
	applySortQuery(&pager, sortField, sortAsc)
	noticeKind, noticeText := noticeFromQuery(r.URL.Query())
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := s.pageShell()
	mergeActorData(data, ac)
	data["Items"] = pageItems
	data["StatsLine"] = statsLine(len(items), total)
	data["Summary"] = map[string]any{
		"Count":           len(allItems),
		"Bytes":           total,
		"ClusterCount":    len(store.ClusterCounts(allItems)),
		"IncompleteCount": incomplete,
		"StorageTopology": topo,
	}
	data["Filter"] = filter
	data["ClusterChips"] = store.ClusterCounts(allItems)
	data["FilterURL"] = FilterURLBuilder{Base: r.URL.RawQuery}
	data["MaxUpload"] = s.maxUpload()
	data["Pager"] = pager
	data["NoticeKind"] = noticeKind
	data["NoticeText"] = noticeText
	data["Nav"] = "captures"
	data["BaseURL"] = requestBaseURL(r)
	if ac != nil && ac.User.ID != 0 {
		pins, perr := s.Store.ListPins(r.Context(), ac.User.ID, 16)
		if perr != nil {
			slog.Warn("list pins", "user", ac.User.ID, "error", perr)
		} else {
			data["Pins"] = pins
		}
	}
	if err := homeTmpl.Execute(w, data); err != nil {
		slog.Error("render home", "error", err)
	}
}

func (s *Server) handleUploadGET(w http.ResponseWriter, r *http.Request) {
	ac := actorFrom(r.Context())
	noticeKind, noticeText := noticeFromQuery(r.URL.Query())
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := s.pageShell()
	mergeActorData(data, ac)
	data["MaxUpload"] = s.maxUpload()
	data["NoticeKind"] = noticeKind
	data["NoticeText"] = noticeText
	data["Nav"] = "upload"
	_ = uploadTmpl.Execute(w, data)
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
	ac := actorFrom(r.Context())
	r.Body = http.MaxBytesReader(w, r.Body, s.maxUpload())
	src, key, err := uploadReader(r)
	if err != nil {
		status := http.StatusBadRequest
		notice := "upload_error"
		if isMaxBytes(err) {
			status = http.StatusRequestEntityTooLarge
			notice = "too_large"
		}
		if isBrowserForm(r) {
			http.Redirect(w, r, "/upload?notice="+notice, http.StatusSeeOther)
			return
		}
		writeJSONError(w, status, "bad_request")
		return
	}
	defer func() { _ = src.Close() }()
	a, err := s.ingestBody(r.Context(), src, key, ac.User.ID)
	if err != nil {
		var dup *store.DuplicateError
		if errors.As(err, &dup) {
			if isBrowserForm(r) {
				http.Redirect(w, r, "/upload?notice=duplicate&name="+url.QueryEscape(store.SanitizeArchiveKey(key)), http.StatusSeeOther)
				return
			}
			writeJSONDuplicate(w, dup.Existing)
			return
		}
		if isBrowserForm(r) {
			notice := "upload_error"
			if isMaxBytes(err) {
				notice = "too_large"
			}
			http.Redirect(w, r, "/upload?notice="+notice, http.StatusSeeOther)
			return
		}
		if isMaxBytes(err) {
			writeJSONError(w, http.StatusRequestEntityTooLarge, "too_large")
			return
		}
		slog.Error("ingest failed", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	s.recordAudit(r, "upload", a)
	if isBrowserForm(r) {
		http.Redirect(w, r, "/upload?notice=uploaded&name="+url.QueryEscape(a.Key), http.StatusSeeOther)
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
	s.recordAudit(r, "download", a)
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
