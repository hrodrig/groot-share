package server

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hrodrig/groot-share/internal/blob"
	"github.com/hrodrig/groot-share/internal/config"
	"github.com/hrodrig/groot-share/internal/store"
)

// countingBlobs wraps blob.Memory and counts List calls.
type countingBlobs struct {
	*blob.Memory
	lists atomic.Int64
}

func (c *countingBlobs) List(ctx context.Context, prefix string) ([]blob.Object, error) {
	c.lists.Add(1)
	return c.Memory.List(ctx, prefix)
}

func newCachedServer(t *testing.T) (*Server, *store.Store, *countingBlobs) {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	blobs := &countingBlobs{Memory: blob.NewMemory()}
	s := &Server{
		Cfg:   config.Config{Topology: config.TopologyVPSS3, S3Prefix: "prefix/"},
		Store: st,
		Blobs: blobs,
		Ready: func() bool { return true },
	}
	// Seed one object so List has something to return.
	if err := blobs.Put(context.Background(), "prefix/2026/obj.tar.gz", strings.NewReader("x")); err != nil {
		t.Fatal(err)
	}
	return s, st, blobs
}

func TestListItemsBucketCachesWithinTTL(t *testing.T) {
	s, _, blobs := newCachedServer(t)

	if _, err := s.listItems(context.Background()); err != nil {
		t.Fatalf("first list: %v", err)
	}
	if _, err := s.listItems(context.Background()); err != nil {
		t.Fatalf("second list: %v", err)
	}
	if got := blobs.lists.Load(); got != 1 {
		t.Fatalf("want 1 List call within TTL, got %d", got)
	}
}

func TestListItemsBucketInvalidatesOnUpload(t *testing.T) {
	s, _, blobs := newCachedServer(t)

	if _, err := s.listItems(context.Background()); err != nil {
		t.Fatal(err)
	}
	// A successful bucket upload invalidates the cache.
	s.listCache.invalidate()
	if _, err := s.listItems(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := blobs.lists.Load(); got != 2 {
		t.Fatalf("want 2 List calls after invalidate, got %d", got)
	}
}

func TestListItemsBucketExpiresAfterTTL(t *testing.T) {
	s, _, blobs := newCachedServer(t)

	if _, err := s.listItems(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Age the cache past TTL without exercising real time sleeps.
	s.listCache.mu.Lock()
	s.listCache.filled = time.Now().Add(-listCacheTTL - time.Second)
	s.listCache.mu.Unlock()

	if _, err := s.listItems(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := blobs.lists.Load(); got != 2 {
		t.Fatalf("want 2 List calls after TTL expiry, got %d", got)
	}
}
