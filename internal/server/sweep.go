package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hrodrig/groot-share/internal/blob"
	"github.com/hrodrig/groot-share/internal/retain"
	"github.com/hrodrig/groot-share/internal/store"
)

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost && !strings.HasSuffix(strings.Trim(r.PathValue("id"), "/"), "/delete") {
		http.NotFound(w, r)
		return
	}
	id := downloadID(r)
	a, err := s.removeArchive(r.Context(), id)
	if err != nil {
		if r.Method == http.MethodDelete || wantsJSON(r) {
			writeJSONError(w, http.StatusNotFound, "not_found")
			return
		}
		http.NotFound(w, r)
		return
	}
	s.recordAudit(r, "delete", a)
	if r.Method == http.MethodDelete || wantsJSON(r) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, "/?notice=deleted", http.StatusSeeOther)
}

func (s *Server) removeArchive(ctx context.Context, id string) (store.Archive, error) {
	if id == "" || strings.Contains(id, "..") {
		return store.Archive{}, store.ErrNotFound
	}
	if s.useBucket() {
		return s.removeBucket(ctx, id)
	}
	a, err := s.Store.ArchiveByID(ctx, id)
	if err != nil {
		return store.Archive{}, err
	}
	if err := s.Store.DeleteArchive(ctx, id); err != nil {
		return store.Archive{}, err
	}
	return a, nil
}

func (s *Server) removeBucket(ctx context.Context, id string) (store.Archive, error) {
	if !blob.UnderPrefix(s.Cfg.S3Prefix, id) {
		return store.Archive{}, store.ErrNotFound
	}
	obj, err := s.Blobs.Head(ctx, id)
	if err == nil {
		if err := s.Blobs.Delete(ctx, id); err != nil {
			return store.Archive{}, err
		}
		return objectArchive(obj), nil
	}
	if !errors.Is(err, blob.ErrNotFound) {
		return store.Archive{}, err
	}
	tr, err := s.Store.TransitByS3Key(ctx, id)
	if err != nil {
		return store.Archive{}, store.ErrNotFound
	}
	if err := s.Store.DeleteTransit(ctx, tr.ID); err != nil {
		return store.Archive{}, err
	}
	return store.Archive{ID: tr.S3Key, Key: tr.Key, Size: tr.Size, Source: "http", Storage: "transit"}, nil
}

// SweepOnce deletes home objects that fail keep_last or max_age, then staging leftovers.
func (s *Server) SweepOnce(ctx context.Context) error {
	now := time.Now().UTC()
	if n, err := s.Store.PurgeExpiredSessions(ctx, now); err != nil {
		slog.Error("purge sessions", "error", err)
	} else if n > 0 {
		slog.Info("purged expired sessions", "count", n)
	}
	items, err := s.listItems(ctx)
	if err != nil {
		return err
	}
	for _, a := range retain.Pick(items, s.Cfg.KeepLast, s.Cfg.MaxAgeDays, now) {
		if _, err := s.removeArchive(ctx, a.ID); err != nil {
			slog.Warn("retention delete failed", "id", a.ID, "error", err)
			continue
		}
		if err := s.Store.InsertAudit(ctx, store.Audit{
			Actor: "retention", Action: "delete", ObjectID: a.ID, ObjectKey: a.Key,
		}); err != nil {
			slog.Error("audit insert", "error", err, "action", "delete")
		}
	}
	s.sweepStaging(ctx, now)
	return nil
}

// SweepLoop runs SweepOnce until ctx is done.
func (s *Server) SweepLoop(ctx context.Context, every time.Duration) {
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
			if err := s.SweepOnce(ctx); err != nil {
				slog.Warn("retention sweep", "error", err)
			}
		}
	}
}

func (s *Server) sweepStaging(ctx context.Context, now time.Time) {
	grace := s.Cfg.StagingGrace
	if grace <= 0 {
		grace = 24 * time.Hour
	}
	items, err := s.Store.ListTransit(ctx)
	if err != nil {
		slog.Warn("list transit", "error", err)
		return
	}
	for _, tr := range items {
		if now.Sub(tr.CreatedAt) < grace {
			continue
		}
		slog.Error("staging leftover swept", "id", tr.ID, "key", tr.Key, "s3_key", tr.S3Key, "last_error", tr.LastError)
		if err := s.Store.DeleteTransit(ctx, tr.ID); err != nil {
			slog.Warn("staging sweep", "error", err)
		}
	}
	entries, err := os.ReadDir(s.Store.StagingDir())
	if err != nil {
		return
	}
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || now.Sub(info.ModTime()) < grace {
			continue
		}
		slog.Warn("staging leftover swept", "name", e.Name())
		_ = os.Remove(filepath.Join(s.Store.StagingDir(), e.Name()))
	}
}
