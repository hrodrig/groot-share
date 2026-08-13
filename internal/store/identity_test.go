package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hrodrig/groot-share/internal/auth"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func TestEnsureAdminThenNoOp(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	if err := st.EnsureAdmin(ctx, "", ""); err == nil {
		t.Fatal("empty table without env should fail")
	}
	if err := st.EnsureAdmin(ctx, "root", "short"); err == nil {
		t.Fatal("short password")
	}
	if err := st.EnsureAdmin(ctx, "root", "correct-horse"); err != nil {
		t.Fatal(err)
	}
	if err := st.EnsureAdmin(ctx, "other", "another-pass"); err != nil {
		t.Fatal(err)
	}
	n, err := st.UserCount(ctx)
	if err != nil || n != 1 {
		t.Fatalf("count %d %v", n, err)
	}
	u, err := st.UserByUsername(ctx, "root")
	if err != nil || !u.Admin {
		t.Fatalf("user %+v %v", u, err)
	}
}

func TestSessionRoundTrip(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	hash, err := auth.HashPassword("correct-horse")
	if err != nil {
		t.Fatal(err)
	}
	u, err := st.CreateUser(ctx, "alice", hash, false)
	if err != nil {
		t.Fatal(err)
	}
	raw, shash, err := auth.NewSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateSession(ctx, u.ID, shash, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	got, err := st.UserBySessionHash(ctx, auth.HashSecret(raw))
	if err != nil || got.Username != "alice" {
		t.Fatalf("sess %+v %v", got, err)
	}
	if err := st.DeleteSession(ctx, shash); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UserBySessionHash(ctx, shash); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted: %v", err)
	}
}

func TestAPIKeyStoredHashed(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	hash, err := auth.HashPassword("correct-horse")
	if err != nil {
		t.Fatal(err)
	}
	u, err := st.CreateUser(ctx, "alice", hash, false)
	if err != nil {
		t.Fatal(err)
	}
	kraw, khash, prefix, err := auth.NewAPIKey()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateAPIKey(ctx, u.ID, khash, prefix); err != nil {
		t.Fatal(err)
	}
	got, err := st.UserByAPIKeyHash(ctx, auth.HashSecret(kraw))
	if err != nil || got.ID != u.ID {
		t.Fatalf("key %+v %v", got, err)
	}
	ok, err := st.APIKeyHashStored(ctx, khash)
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	if kraw == khash {
		t.Fatal("raw key must not be stored as hash")
	}
}
