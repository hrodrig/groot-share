package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
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

func (c *Core) IngestBody(ctx context.Context, r io.Reader, key string, uploadedBy int64) (store.Archive, error) {
	return c.IngestWithSource(ctx, r, key, "http", uploadedBy)
}

func (c *Core) IngestWithSource(ctx context.Context, r io.Reader, key, source string, uploadedBy int64) (store.Archive, error) {
	if c.UseBucket() {
		return c.ingestTransit(ctx, r, key, source, uploadedBy)
	}
	st, err := c.Store.Stage(ctx, r, key, uploadedBy)
	if err != nil {
		return store.Archive{}, err
	}
	if existing, err := c.Store.FindExistingSHA256(ctx, st.SHA256); err == nil {
		_ = os.Remove(st.Path)
		return store.Archive{}, &store.DuplicateError{Existing: existing}
	} else if !errors.Is(err, store.ErrNotFound) {
		_ = os.Remove(st.Path)
		return store.Archive{}, err
	}
	return c.Store.CommitLocal(ctx, st, source)
}

func (c *Core) UseBucket() bool {
	return c.Blobs != nil && c.Cfg.Topology == config.TopologyVPSS3
}

func (c *Core) ingestTransit(ctx context.Context, r io.Reader, key, source string, uploadedBy int64) (store.Archive, error) {
	now := time.Now().UTC()
	var id, s3key string
	var err error
	if source == "sftp" {
		id, s3key, err = c.uniqueSFTPKey(ctx, now)
	} else {
		id, s3key, err = c.UniqueHTTPKey(ctx, now)
	}
	if err != nil {
		return store.Archive{}, err
	}
	st, err := c.Store.StageWithID(ctx, id, r, key, uploadedBy)
	if err != nil {
		return store.Archive{}, err
	}
	if existing, err := c.Store.FindExistingSHA256(ctx, st.SHA256); err == nil {
		_ = os.Remove(st.Path)
		return store.Archive{}, &store.DuplicateError{Existing: existing}
	} else if !errors.Is(err, store.ErrNotFound) {
		_ = os.Remove(st.Path)
		return store.Archive{}, err
	}
	return c.copyOrTransit(ctx, st, s3key, source)
}

func (c *Core) UniqueHTTPKey(ctx context.Context, now time.Time) (id, key string, err error) {
	return c.uniqueObjectKey(ctx, now, func(id string, t time.Time) string {
		return blob.HTTPKey(c.Cfg.S3Prefix, id, t)
	})
}

func (c *Core) uniqueSFTPKey(ctx context.Context, now time.Time) (id, key string, err error) {
	return c.uniqueObjectKey(ctx, now, func(id string, t time.Time) string {
		return blob.SFTPKey(c.Cfg.S3Prefix, id, t)
	})
}

func (c *Core) uniqueObjectKey(ctx context.Context, now time.Time, makeKey func(id string, t time.Time) string) (id, key string, err error) {
	for range 8 {
		id, err = store.NewArchiveID()
		if err != nil {
			return "", "", fmt.Errorf("id: %w", err)
		}
		key = makeKey(id, now)
		_, err = c.Blobs.Head(ctx, key)
		if errors.Is(err, blob.ErrNotFound) {
			return id, key, nil
		}
		if err != nil {
			return "", "", fmt.Errorf("head: %w", err)
		}
	}
	return "", "", fmt.Errorf("could not allocate object key")
}

func (c *Core) copyOrTransit(ctx context.Context, st store.Staged, s3key, source string) (store.Archive, error) {
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
	if err := c.putFile(ctx, s3key, st.Path); err != nil {
		if saveErr := c.Store.SaveTransit(ctx, st, s3key, err.Error()); saveErr != nil {
			return store.Archive{}, fmt.Errorf("save transit: %w", saveErr)
		}
		a.Storage = "transit"
		return a, nil
	}
	_ = os.Remove(st.Path)
	if err := c.Store.InsertArchiveMeta(ctx, a); err != nil {
		slog.Warn("archive meta index", "error", err, "id", a.ID)
	}
	c.listCache.invalidate()
	c.completenessCache.invalidate()
	return a, nil
}

func (c *Core) putFile(ctx context.Context, key, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open staging: %w", err)
	}
	defer func() { _ = f.Close() }()
	return c.Blobs.Put(ctx, key, f)
}

// RetryOnce attempts one Put per transit row.
func (c *Core) RetryOnce(ctx context.Context) error {
	if !c.UseBucket() {
		return nil
	}
	items, err := c.Store.ListTransit(ctx)
	if err != nil {
		return err
	}
	for _, tr := range items {
		if err := c.putFile(ctx, tr.S3Key, tr.Path); err != nil {
			_ = c.Store.UpdateTransitError(ctx, tr.ID, err.Error())
			slog.Warn("transit retry failed", "key", tr.S3Key, "error", err)
			continue
		}
		if err := c.Store.DeleteTransit(ctx, tr.ID); err != nil {
			slog.Error("transit cleanup", "error", err)
		}
		c.listCache.invalidate()
	}
	return nil
}

// RetryLoop copies transit objects until ctx is done.
func (c *Core) RetryLoop(ctx context.Context, every time.Duration) {
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
			if err := c.RetryOnce(ctx); err != nil {
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

// ListCacheTTL bounds staleness of the cached listing.
const ListCacheTTL = 5 * time.Second

func (c *listCache) get(now time.Time) ([]store.Archive, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.filled.IsZero() || now.Sub(c.filled) >= ListCacheTTL {
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

// InvalidateListCache drops the vps-s3 listing memo (tests and mutations).
func (c *Core) InvalidateListCache() { c.listCache.invalidate() }

// SetListCacheFilledForTest ages the list cache timestamp (server package tests).
func (c *Core) SetListCacheFilledForTest(t time.Time) {
	c.listCache.mu.Lock()
	defer c.listCache.mu.Unlock()
	c.listCache.filled = t
}

func (c *listCache) invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = nil
	c.filled = time.Time{}
}

func (c *Core) ListItems(ctx context.Context) ([]store.Archive, error) {
	if !c.UseBucket() {
		return c.Store.ListArchives(ctx)
	}
	return c.listItemsBucket(ctx)
}

func (c *Core) listItemsBucket(ctx context.Context) ([]store.Archive, error) {
	now := time.Now()
	if items, ok := c.listCache.get(now); ok {
		return items, nil
	}
	prefix := blob.NormalizePrefix(c.Cfg.S3Prefix)
	objs, err := c.Blobs.List(ctx, prefix)
	if err != nil {
		return nil, err
	}
	out := make([]store.Archive, 0, len(objs))
	for _, o := range objs {
		out = append(out, objectArchive(o))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID > out[j].ID
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	c.listCache.set(out, now)
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

func (c *Core) OpenDownload(ctx context.Context, id string) (io.ReadCloser, store.Archive, error) {
	if id == "" || strings.Contains(id, "..") {
		return nil, store.Archive{}, store.ErrNotFound
	}
	if c.UseBucket() {
		return c.openVPSS3(ctx, id)
	}
	a, err := c.Store.ArchiveByID(ctx, id)
	if err != nil {
		return nil, store.Archive{}, err
	}
	p, err := c.Store.BlobPath(a.ID)
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

func (c *Core) openVPSS3(ctx context.Context, id string) (io.ReadCloser, store.Archive, error) {
	if !blob.UnderPrefix(c.Cfg.S3Prefix, id) {
		return nil, store.Archive{}, store.ErrNotFound
	}
	rc, obj, err := c.Blobs.Get(ctx, id)
	if err == nil {
		return rc, objectArchive(obj), nil
	}
	if !errors.Is(err, blob.ErrNotFound) {
		return nil, store.Archive{}, err
	}
	tr, err := c.Store.TransitByS3Key(ctx, id)
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

// resolveArchive returns the archive metadata for id, transparently
// handling both vps (32-hex opaque id) and vps-s3 (S3 object key,
// which may contain '/'). It is the share-flow counterpart to
// openDownload (which additionally opens a reader for the body).
func (c *Core) ResolveArchive(ctx context.Context, id string) (store.Archive, error) {
	if id == "" || strings.Contains(id, "..") {
		return store.Archive{}, store.ErrNotFound
	}
	if c.UseBucket() {
		return c.resolveArchiveVPSS3(ctx, id)
	}
	return c.Store.ArchiveByID(ctx, id)
}

// resolveArchiveVPSS3 mirrors openVPSS3 but skips the body read; the
// share handlers only need the archive metadata.
func (c *Core) resolveArchiveVPSS3(ctx context.Context, id string) (store.Archive, error) {
	if !blob.UnderPrefix(c.Cfg.S3Prefix, id) {
		return store.Archive{}, store.ErrNotFound
	}
	obj, err := c.Blobs.Head(ctx, id)
	if err == nil {
		return objectArchive(obj), nil
	}
	if !errors.Is(err, blob.ErrNotFound) {
		return store.Archive{}, err
	}
	tr, err := c.Store.TransitByS3Key(ctx, id)
	if err != nil {
		return store.Archive{}, store.ErrNotFound
	}
	return store.Archive{
		ID:        tr.S3Key,
		Key:       tr.Key,
		Size:      tr.Size,
		SHA256:    tr.SHA256,
		Source:    blob.SourceForKey(tr.S3Key),
		Storage:   "transit",
		CreatedAt: tr.CreatedAt,
	}, nil
}
