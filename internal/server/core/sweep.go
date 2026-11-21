package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hrodrig/groot-share/internal/blob"
	"github.com/hrodrig/groot-share/internal/retain"
	"github.com/hrodrig/groot-share/internal/store"
)

func (c *Core) RemoveArchive(ctx context.Context, id string) (store.Archive, error) {
	if id == "" || strings.Contains(id, "..") {
		return store.Archive{}, store.ErrNotFound
	}
	if c.UseBucket() {
		return c.removeBucket(ctx, id)
	}
	a, err := c.Store.ArchiveByID(ctx, id)
	if err != nil {
		return store.Archive{}, err
	}
	if err := c.Store.DeleteArchive(ctx, id); err != nil {
		return store.Archive{}, err
	}
	return a, nil
}

func (c *Core) removeBucket(ctx context.Context, id string) (store.Archive, error) {
	if !blob.UnderPrefix(c.Cfg.S3Prefix, id) {
		return store.Archive{}, store.ErrNotFound
	}
	obj, err := c.Blobs.Head(ctx, id)
	if err == nil {
		if err := c.Blobs.Delete(ctx, id); err != nil {
			return store.Archive{}, err
		}
		c.listCache.invalidate()
		c.completenessCache.invalidate()
		// Drop share links and pins for the removed object (#39). The S3
		// object has no `archives` row, so clean dependents explicitly.
		if err := c.Store.DeleteArchiveDependents(ctx, id); err != nil {
			slog.Warn("delete dependents", "id", id, "error", err)
		}
		return objectArchive(obj), nil
	}
	if !errors.Is(err, blob.ErrNotFound) {
		return store.Archive{}, err
	}
	tr, err := c.Store.TransitByS3Key(ctx, id)
	if err != nil {
		return store.Archive{}, store.ErrNotFound
	}
	if err := c.Store.DeleteTransit(ctx, tr.ID); err != nil {
		return store.Archive{}, err
	}
	if err := c.Store.DeleteArchiveDependents(ctx, tr.S3Key); err != nil {
		slog.Warn("delete dependents", "id", tr.S3Key, "error", err)
	}
	return store.Archive{ID: tr.S3Key, Key: tr.Key, Size: tr.Size, Source: blob.SourceForKey(tr.S3Key), Storage: "transit"}, nil
}

// SweepOnce deletes home objects that fail keep_last or max_age, then staging leftovers.
func (c *Core) SweepOnce(ctx context.Context) error {
	now := time.Now().UTC()
	if n, err := c.Store.PurgeExpiredSessions(ctx, now); err != nil {
		slog.Error("purge sessions", "error", err)
	} else if n > 0 {
		slog.Info("purged expired sessions", "count", n)
	}
	items, err := c.ListItems(ctx)
	if err != nil {
		return err
	}
	protected, err := c.Store.ProtectedArchiveIDs(ctx, now)
	if err != nil {
		// Fail closed: if we can't tell what's pinned/shared, don't delete
		// anything this cycle rather than risk deleting a protected archive.
		return fmt.Errorf("retention protected set: %w", err)
	}
	for _, a := range retain.Pick(items, c.Cfg.KeepLast, c.Cfg.MaxAgeDays, now) {
		if _, ok := protected[a.ID]; ok {
			// Pinned or actively shared — retention must not delete it (#45).
			continue
		}
		if _, err := c.RemoveArchive(ctx, a.ID); err != nil {
			slog.Warn("retention delete failed", "id", a.ID, "error", err)
			continue
		}
		if err := c.Store.InsertAudit(ctx, store.Audit{
			Actor: "retention", Action: "delete", ObjectID: a.ID, ObjectKey: a.Key,
		}); err != nil {
			slog.Error("audit insert", "error", err, "action", "delete")
		}
	}
	c.sweepStaging(ctx, now)
	return nil
}

// SweepLoop runs SweepOnce until ctx is done.
func (c *Core) SweepLoop(ctx context.Context, every time.Duration) {
	if every <= 0 {
		every = time.Hour
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := c.SweepOnce(ctx); err != nil {
				slog.Warn("retention sweep", "error", err)
			}
		}
	}
}

func (c *Core) sweepStaging(ctx context.Context, now time.Time) {
	grace := c.Cfg.StagingGrace
	if grace <= 0 {
		grace = 24 * time.Hour
	}
	items, err := c.Store.ListTransit(ctx)
	if err != nil {
		slog.Warn("list transit", "error", err)
		return
	}
	for _, tr := range items {
		if now.Sub(tr.CreatedAt) < grace {
			continue
		}
		slog.Error("staging leftover swept", "id", tr.ID, "key", tr.Key, "s3_key", tr.S3Key, "last_error", tr.LastError)
		if err := c.Store.DeleteTransit(ctx, tr.ID); err != nil {
			slog.Warn("staging sweep", "error", err)
		}
	}
	entries, err := os.ReadDir(c.Store.StagingDir())
	if err != nil {
		return
	}
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || now.Sub(info.ModTime()) < grace {
			continue
		}
		slog.Warn("staging leftover swept", "name", e.Name())
		_ = os.Remove(filepath.Join(c.Store.StagingDir(), e.Name()))
	}
}
