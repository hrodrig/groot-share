package store

import (
	"context"
	"database/sql"
	"errors"
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
	n, err := st.IncrementShareUse(ctx, link.ID)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("after 1 use = %d, want 1", n)
	}
	n, err = st.IncrementShareUse(ctx, link.ID)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("after 2 uses = %d, want 2", n)
	}
}

func TestIncrementShareUseNotFound(t *testing.T) {
	st := archiveStore(t)
	if _, err := st.IncrementShareUse(context.Background(), 999999); !errors.Is(err, errShareNotFound) {
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
