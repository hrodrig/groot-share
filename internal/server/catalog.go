package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/hrodrig/groot-share/internal/blob"
	"github.com/hrodrig/groot-share/internal/config"
	"github.com/hrodrig/groot-share/internal/store"
)

func (s *Server) ingestBody(ctx context.Context, r io.Reader, key string, uploadedBy int64) (store.Archive, error) {
	return s.ingestWithSource(ctx, r, key, "http", uploadedBy)
}

func (s *Server) ingestWithSource(ctx context.Context, r io.Reader, key, source string, uploadedBy int64) (store.Archive, error) {
	if s.useBucket() {
		return s.ingestTransit(ctx, r, key, source, uploadedBy)
	}
	st, err := s.Store.Stage(ctx, r, key, uploadedBy)
	if err != nil {
		return store.Archive{}, err
	}
	if existing, err := s.Store.FindExistingSHA256(ctx, st.SHA256); err == nil {
		_ = os.Remove(st.Path)
		return store.Archive{}, &store.DuplicateError{Existing: existing}
	} else if !errors.Is(err, store.ErrNotFound) {
		_ = os.Remove(st.Path)
		return store.Archive{}, err
	}
	return s.Store.CommitLocal(ctx, st, source)
}

func (s *Server) useBucket() bool {
	return s.Blobs != nil && s.Cfg.Topology == config.TopologyVPSS3
}

func (s *Server) ingestTransit(ctx context.Context, r io.Reader, key, source string, uploadedBy int64) (store.Archive, error) {
	now := time.Now().UTC()
	var id, s3key string
	var err error
	if source == "sftp" {
		id, s3key, err = s.uniqueSFTPKey(ctx, now)
	} else {
		id, s3key, err = s.uniqueHTTPKey(ctx, now)
	}
	if err != nil {
		return store.Archive{}, err
	}
	st, err := s.Store.StageWithID(ctx, id, r, key, uploadedBy)
	if err != nil {
		return store.Archive{}, err
	}
	if existing, err := s.Store.FindExistingSHA256(ctx, st.SHA256); err == nil {
		_ = os.Remove(st.Path)
		return store.Archive{}, &store.DuplicateError{Existing: existing}
	} else if !errors.Is(err, store.ErrNotFound) {
		_ = os.Remove(st.Path)
		return store.Archive{}, err
	}
	return s.copyOrTransit(ctx, st, s3key, source)
}

func (s *Server) uniqueHTTPKey(ctx context.Context, now time.Time) (id, key string, err error) {
	return s.uniqueObjectKey(ctx, now, func(id string, t time.Time) string {
		return blob.HTTPKey(s.Cfg.S3Prefix, id, t)
	})
}

func (s *Server) uniqueSFTPKey(ctx context.Context, now time.Time) (id, key string, err error) {
	return s.uniqueObjectKey(ctx, now, func(id string, t time.Time) string {
		return blob.SFTPKey(s.Cfg.S3Prefix, id, t)
	})
}

func (s *Server) uniqueObjectKey(ctx context.Context, now time.Time, makeKey func(id string, t time.Time) string) (id, key string, err error) {
	for range 8 {
		id, err = store.NewArchiveID()
		if err != nil {
			return "", "", fmt.Errorf("id: %w", err)
		}
		key = makeKey(id, now)
		_, err = s.Blobs.Head(ctx, key)
		if errors.Is(err, blob.ErrNotFound) {
			return id, key, nil
		}
		if err != nil {
			return "", "", fmt.Errorf("head: %w", err)
		}
	}
	return "", "", fmt.Errorf("could not allocate object key")
}

func (s *Server) copyOrTransit(ctx context.Context, st store.Staged, s3key, source string) (store.Archive, error) {
	if source == "" {
		source = "http"
	}
	a := store.Archive{
		ID:         s3key,
		Key:        st.Key,
		Size:       st.Size,
		SHA256:     st.SHA256,
		Source:     source,
		Storage:    "s3",
		UploadedBy: st.UploadedBy,
		CreatedAt:  st.CreatedAt,
	}
	if err := s.putFile(ctx, s3key, st.Path); err != nil {
		if saveErr := s.Store.SaveTransit(ctx, st, s3key, err.Error()); saveErr != nil {
			return store.Archive{}, fmt.Errorf("save transit: %w", saveErr)
		}
		a.Storage = "transit"
		return a, nil
	}
	_ = os.Remove(st.Path)
	if err := s.Store.InsertArchiveMeta(ctx, a); err != nil {
		slog.Warn("archive meta index", "error", err, "id", a.ID)
	}
	s.listCache.invalidate()
	return a, nil
}

func (s *Server) putFile(ctx context.Context, key, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open staging: %w", err)
	}
	defer func() { _ = f.Close() }()
	return s.Blobs.Put(ctx, key, f)
}

// RetryOnce attempts one Put per transit row.
func (s *Server) RetryOnce(ctx context.Context) error {
	if !s.useBucket() {
		return nil
	}
	items, err := s.Store.ListTransit(ctx)
	if err != nil {
		return err
	}
	for _, tr := range items {
		if err := s.putFile(ctx, tr.S3Key, tr.Path); err != nil {
			_ = s.Store.UpdateTransitError(ctx, tr.ID, err.Error())
			slog.Warn("transit retry failed", "key", tr.S3Key, "error", err)
			continue
		}
		if err := s.Store.DeleteTransit(ctx, tr.ID); err != nil {
			slog.Error("transit cleanup", "error", err)
		}
		s.listCache.invalidate()
	}
	return nil
}

// RetryLoop copies transit objects until ctx is done.
func (s *Server) RetryLoop(ctx context.Context, every time.Duration) {
	if every <= 0 {
		every = 30 * time.Second
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := s.RetryOnce(ctx); err != nil {
				slog.Warn("transit retry", "error", err)
			}
		}
	}
}

// listCache caches the vps-s3 object listing to avoid a full ListObjects on
// every Captures page / GET /v1/archives. Zero-value is a cold, thread-safe
// cache; it is only consulted when useBucket() is true.
type listCache struct {
	mu     sync.Mutex
	items  []store.Archive
	filled time.Time
}

// listCacheTTL bounds staleness of the cached listing.
const listCacheTTL = 5 * time.Second

func (c *listCache) get(now time.Time) ([]store.Archive, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.filled.IsZero() || now.Sub(c.filled) >= listCacheTTL {
		return nil, false
	}
	return c.items, true
}

func (c *listCache) set(items []store.Archive, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = items
	c.filled = now
}

func (c *listCache) invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = nil
	c.filled = time.Time{}
}

func (s *Server) listItems(ctx context.Context) ([]store.Archive, error) {
	if !s.useBucket() {
		return s.Store.ListArchives(ctx)
	}
	return s.listItemsBucket(ctx)
}

func (s *Server) listItemsBucket(ctx context.Context) ([]store.Archive, error) {
	now := time.Now()
	if items, ok := s.listCache.get(now); ok {
		return items, nil
	}
	prefix := blob.NormalizePrefix(s.Cfg.S3Prefix)
	objs, err := s.Blobs.List(ctx, prefix)
	if err != nil {
		return nil, err
	}
	out := make([]store.Archive, 0, len(objs))
	for _, o := range objs {
		out = append(out, objectArchive(o))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	s.listCache.set(out, now)
	return out, nil
}

func objectArchive(o blob.Object) store.Archive {
	return store.Archive{
		ID:        o.Key,
		Key:       filepath.Base(o.Key),
		Size:      o.Size,
		SHA256:    o.ETag,
		Source:    blob.SourceForKey(o.Key),
		Storage:   "s3",
		CreatedAt: o.LastModified,
	}
}

func (s *Server) openDownload(ctx context.Context, id string) (io.ReadCloser, store.Archive, error) {
	if id == "" || strings.Contains(id, "..") {
		return nil, store.Archive{}, store.ErrNotFound
	}
	if s.useBucket() {
		return s.openVPSS3(ctx, id)
	}
	a, err := s.Store.ArchiveByID(ctx, id)
	if err != nil {
		return nil, store.Archive{}, err
	}
	p, err := s.Store.BlobPath(a.ID)
	if err != nil {
		return nil, store.Archive{}, err
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, store.Archive{}, store.ErrNotFound
	}
	if a.Storage == "" {
		a.Storage = "local"
	}
	return f, a, nil
}

func (s *Server) openVPSS3(ctx context.Context, id string) (io.ReadCloser, store.Archive, error) {
	if !blob.UnderPrefix(s.Cfg.S3Prefix, id) {
		return nil, store.Archive{}, store.ErrNotFound
	}
	rc, obj, err := s.Blobs.Get(ctx, id)
	if err == nil {
		return rc, objectArchive(obj), nil
	}
	if !errors.Is(err, blob.ErrNotFound) {
		return nil, store.Archive{}, err
	}
	tr, err := s.Store.TransitByS3Key(ctx, id)
	if err != nil {
		return nil, store.Archive{}, store.ErrNotFound
	}
	f, err := os.Open(tr.Path)
	if err != nil {
		return nil, store.Archive{}, store.ErrNotFound
	}
	return f, store.Archive{
		ID:        tr.S3Key,
		Key:       tr.Key,
		Size:      tr.Size,
		SHA256:    tr.SHA256,
		Source:    blob.SourceForKey(tr.S3Key),
		Storage:   "transit",
		CreatedAt: tr.CreatedAt,
	}, nil
}

func downloadID(r *http.Request) string {
	id := strings.Trim(r.PathValue("id"), "/")
	id = strings.TrimSuffix(id, "/file")
	id = strings.TrimSuffix(id, "/delete")
	return strings.TrimSuffix(id, "/")
}
