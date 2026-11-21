package store

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestRevokeShareLink(t *testing.T) {
	st := archiveStore(t)
	ctx := context.Background()
	link, err := st.CreateShareLink(ctx, "arch-1", "hash-1", 1, "label", 0, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.RevokeShareLink(ctx, link.ID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	got, err := st.ShareByTokenHash(ctx, "hash-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.RevokedAt.IsZero() {
		t.Fatal("expected revoked_at set")
	}
	// Revoking again returns not found (revoked_at already set).
	if err := st.RevokeShareLink(ctx, link.ID, time.Now().UTC()); !errors.Is(err, errShareNotFound) {
		t.Fatalf("second revoke = %v, want errShareNotFound", err)
	}
}

func TestRevokeShareLinkNotFound(t *testing.T) {
	st := archiveStore(t)
	if err := st.RevokeShareLink(context.Background(), 999999, time.Now().UTC()); !errors.Is(err, errShareNotFound) {
		t.Fatalf("revoke unknown = %v, want errShareNotFound", err)
	}
}

func TestIncrementShareUse(t *testing.T) {
	st := archiveStore(t)
	ctx := context.Background()
	link, err := st.CreateShareLink(ctx, "arch-1", "hash-2", 1, "label", 5, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	n, err := st.IncrementShareUse(ctx, link.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("after 1 use = %d, want 1", n)
	}
	n, err = st.IncrementShareUse(ctx, link.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("after 2 uses = %d, want 2", n)
	}
}

func TestIncrementShareUseExhausted(t *testing.T) {
	st := archiveStore(t)
	ctx := context.Background()
	link, err := st.CreateShareLink(ctx, "arch-1", "hash-exhaust", 1, "label", 2, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if _, err := st.IncrementShareUse(ctx, link.ID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := st.IncrementShareUse(ctx, link.ID, now); err != nil {
		t.Fatal(err)
	}
	// Third use lands on the cap: must be exhausted, not not-found.
	if _, err := st.IncrementShareUse(ctx, link.ID, now); !errors.Is(err, ErrShareExhausted) {
		t.Fatalf("3rd use = %v, want ErrShareExhausted", err)
	}
}

func TestIncrementShareUseExpired(t *testing.T) {
	st := archiveStore(t)
	ctx := context.Background()
	link, err := st.CreateShareLink(ctx, "arch-1", "hash-exp", 1, "label", 0, time.Now().UTC().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	// now is past expires_at, so the atomic bump must refuse.
	if _, err := st.IncrementShareUse(ctx, link.ID, time.Now().UTC()); !errors.Is(err, ErrShareExhausted) {
		t.Fatalf("expired increment = %v, want ErrShareExhausted", err)
	}
}

func TestIncrementShareUseConcurrentCap(t *testing.T) {
	st := archiveStore(t)
	ctx := context.Background()
	link, err := st.CreateShareLink(ctx, "arch-1", "hash-conc", 1, "label", 1, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	// Fire 20 concurrent increments against a max_uses=1 link. Exactly one
	// must win; every other must observe ErrShareExhausted. This is the H1
	// race the atomic WHERE clause closes.
	const nGoroutines = 20
	errs := make([]error, nGoroutines)
	var wg sync.WaitGroup
	wg.Add(nGoroutines)
	// SQLite pool is SetMaxOpenConns(1), so a shared *Store serializes DB
	// access; each goroutine still exercises its own check+increment.
	for i := 0; i < nGoroutines; i++ {
		go func(i int) {
			defer wg.Done()
			_, errs[i] = st.IncrementShareUse(ctx, link.ID, time.Now().UTC())
		}(i)
	}
	wg.Wait()
	var wins, exhausted int
	for _, err := range errs {
		switch {
		case err == nil:
			wins++
		case errors.Is(err, ErrShareExhausted):
			exhausted++
		case errors.Is(err, ErrShareNotFound):
			t.Fatalf("unexpected not-found: %v", err)
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if wins != 1 {
		t.Fatalf("wins = %d, want 1 (max_uses=1)", wins)
	}
	if exhausted != nGoroutines-1 {
		t.Fatalf("exhausted = %d, want %d", exhausted, nGoroutines-1)
	}
}

func TestIncrementShareUseNotFound(t *testing.T) {
	st := archiveStore(t)
	if _, err := st.IncrementShareUse(context.Background(), 999999, time.Now().UTC()); !errors.Is(err, errShareNotFound) {
		t.Fatalf("increment unknown = %v, want errShareNotFound", err)
	}
}

func TestApplySQLitePragmasError(t *testing.T) {
	// A closed DB must make every ExecContext fail, exercising the error paths.
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := applySQLitePragmas(context.Background(), db); err == nil {
		t.Fatal("expected error from closed db")
	}
}

func TestCloseNil(t *testing.T) {
	var st *Store
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	st = &Store{db: nil, dir: "/tmp"}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
}
