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

	"github.com/hrodrig/groot-share/internal/store"
)

// WatchLoop polls GFS_SFTP_INBOX until ctx is done. No-op when inbox is unset.
func (c *Core) WatchLoop(ctx context.Context, every time.Duration) {
	if strings.TrimSpace(c.Cfg.SFTPInbox) == "" {
		return
	}
	if every <= 0 {
		every = 30 * time.Second
	}
	seen := map[string]int64{}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := c.WatchOnce(ctx, seen); err != nil {
				slog.Warn("sftp inbox", "error", err)
			}
		}
	}
}

// WatchOnce ingests stable *.tar.gz files from the inbox (size unchanged vs last poll).
func (c *Core) WatchOnce(ctx context.Context, seen map[string]int64) error {
	inbox := strings.TrimSpace(c.Cfg.SFTPInbox)
	if inbox == "" {
		return nil
	}
	if seen == nil {
		return fmt.Errorf("sftp seen map is nil")
	}
	entries, err := os.ReadDir(inbox)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Warn("sftp inbox missing; skip", "path", inbox)
			return nil
		}
		return fmt.Errorf("read inbox: %w", err)
	}
	live := map[string]struct{}{}
	for _, ent := range entries {
		name := ent.Name()
		path := filepath.Join(inbox, name)
		live[path] = struct{}{}
		c.processInboxEntry(ctx, path, name, ent.IsDir(), seen)
	}
	for path := range seen {
		if _, ok := live[path]; !ok {
			delete(seen, path)
		}
	}
	return nil
}

// processInboxEntry handles one inbox entry: records stability size, then ingests
// stable *.tar.gz files (removing the inbox file on success) and drops stale sizes.
func (c *Core) processInboxEntry(ctx context.Context, path, name string, isDir bool, seen map[string]int64) {
	if isDir || !isInboxTar(name) {
		return
	}
	fi, err := os.Stat(path)
	if err != nil {
		slog.Warn("sftp stat", "path", path, "error", err)
		return
	}
	size := fi.Size()
	prev, ok := seen[path]
	if !ok || prev != size || size <= 0 {
		seen[path] = size
		return
	}
	if err := c.ingestSFTPPath(ctx, path, size); err != nil {
		slog.Warn("sftp ingest", "path", path, "error", err)
		return
	}
	delete(seen, path)
}

func isInboxTar(name string) bool {
	if name == "" || strings.HasPrefix(name, ".") {
		return false
	}
	return strings.HasSuffix(strings.ToLower(name), ".tar.gz")
}

func (c *Core) ingestSFTPPath(ctx context.Context, path string, size int64) error {
	if c.Cfg.MaxUploadBytes > 0 && size > c.Cfg.MaxUploadBytes {
		return fmt.Errorf("file exceeds GFS_MAX_UPLOAD_BYTES")
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	key := store.SanitizeArchiveKey(filepath.Base(path))
	a, err := c.IngestWithSource(ctx, f, key, "sftp", 0)
	if err != nil {
		var dup *store.DuplicateError
		if errors.As(err, &dup) {
			slog.Info("sftp duplicate; drop inbox file", "path", path, "existing", dup.Existing.ID)
			if rmErr := os.Remove(path); rmErr != nil {
				return fmt.Errorf("remove duplicate: %w", rmErr)
			}
			return nil
		}
		return err
	}
	c.recordSFTPAudit("upload", a)
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove inbox file: %w", err)
	}
	slog.Info("sftp ingested", "key", a.Key, "id", a.ID, "source", a.Source)
	return nil
}

func (c *Core) recordSFTPAudit(action string, a store.Archive) {
	if c.Store == nil {
		return
	}
	ev := store.Audit{
		Actor:     "sftp",
		Action:    action,
		ObjectID:  a.ID,
		ObjectKey: a.Key,
	}
	if err := c.Store.InsertAudit(context.Background(), ev); err != nil {
		slog.Error("sftp audit insert", "error", err, "action", action)
	}
}
