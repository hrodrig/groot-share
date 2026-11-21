package store

import (
	"context"
	"errors"
	"github.com/hrodrig/groot-share/internal/auth"
	"testing"
	"time"
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

func archiveStore(t *testing.T) *Store {
	t.Helper()
	st := testStore(t)
	if err := st.EnsureAdmin(context.Background(), "root", "correct-horse", ""); err != nil {
		t.Fatal(err)
	}
	return st
}

func TestEnsureAdminThenNoOp(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	if err := st.EnsureAdmin(ctx, "", "", ""); err == nil {
		t.Fatal("empty table without env should fail")
	}
	if err := st.EnsureAdmin(ctx, "root", "short", ""); err == nil {
		t.Fatal("short password")
	}
	if err := st.EnsureAdmin(ctx, "root", "correct-horse", ""); err != nil {
		t.Fatal(err)
	}
	if err := st.EnsureAdmin(ctx, "other", "another-pass", ""); err != nil {
		t.Fatal(err)
	}
	n, err := st.UserCount(ctx)
	if err != nil || n != 1 {
		t.Fatalf("count %d %v", n, err)
	}
	u, err := st.UserByUsername(ctx, "root")
	if err != nil || u.Role != auth.RoleAdmin {
		t.Fatalf("user %+v %v", u, err)
	}
	if u.Name != DefaultName {
		t.Fatalf("default name %q", u.Name)
	}
}

func TestSessionRoundTrip(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	hash, err := auth.HashPassword("correct-horse")
	if err != nil {
		t.Fatal(err)
	}
	u, err := st.CreateUser(ctx, "alice", "alice", hash, auth.RoleUploader)
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
	u, err := st.CreateUser(ctx, "alice", "alice", hash, auth.RoleUploader)
	if err != nil {
		t.Fatal(err)
	}
	kraw, khash, prefix, err := auth.NewAPIKey()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateAPIKey(ctx, u.ID, khash, prefix, auth.KeyScopeUpload); err != nil {
		t.Fatal(err)
	}
	got, err := st.AuthByAPIKeyHash(ctx, auth.HashSecret(kraw))
	if err != nil || got.User.ID != u.ID || got.Scope != auth.KeyScopeUpload {
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

func TestExpiredSessionRejected(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	hash, err := auth.HashPassword("correct-horse")
	if err != nil {
		t.Fatal(err)
	}
	u, err := st.CreateUser(ctx, "alice", "alice", hash, auth.RoleUploader)
	if err != nil {
		t.Fatal(err)
	}
	raw, shash, err := auth.NewSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateSession(ctx, u.ID, shash, time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UserBySessionHash(ctx, auth.HashSecret(raw)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expired session: %v", err)
	}
}

func TestInactiveUserSessionRejected(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	hash, err := auth.HashPassword("correct-horse")
	if err != nil {
		t.Fatal(err)
	}
	u, err := st.CreateUser(ctx, "alice", "alice", hash, auth.RoleUploader)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateUser(ctx, u.ID, u.Role, false); err != nil {
		t.Fatal(err)
	}
	raw, shash, err := auth.NewSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateSession(ctx, u.ID, shash, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UserBySessionHash(ctx, auth.HashSecret(raw)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("inactive session: %v", err)
	}
}

func TestInactiveUserAPIKeyRejected(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	hash, err := auth.HashPassword("correct-horse")
	if err != nil {
		t.Fatal(err)
	}
	u, err := st.CreateUser(ctx, "alice", "alice", hash, auth.RoleUploader)
	if err != nil {
		t.Fatal(err)
	}
	kraw, khash, prefix, err := auth.NewAPIKey()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateAPIKey(ctx, u.ID, khash, prefix, auth.KeyScopeUpload); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateUser(ctx, u.ID, u.Role, false); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AuthByAPIKeyHash(ctx, auth.HashSecret(kraw)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("inactive api key: %v", err)
	}
}

func TestSetPasswordAndCreateUserValidation(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	hash, err := auth.HashPassword("correct-horse")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateUser(ctx, "  ", "Bob", hash, auth.RoleUploader); !errors.Is(err, ErrUsernameRequired) {
		t.Fatalf("empty username: %v", err)
	}
	if _, err := st.CreateUser(ctx, "bob", "", hash, auth.RoleUploader); !errors.Is(err, ErrNameRequired) {
		t.Fatalf("empty name: %v", err)
	}
	if _, err := st.CreateUser(ctx, "bob", "Bob", hash, auth.Role("superuser")); err == nil {
		t.Fatal("invalid role")
	}
	u, err := st.CreateUser(ctx, "bob", "Bob", hash, auth.RoleViewer)
	if err != nil {
		t.Fatal(err)
	}
	newHash, err := auth.HashPassword("new-secret-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetPassword(ctx, u.ID, newHash); err != nil {
		t.Fatal(err)
	}
	got, err := st.UserByID(ctx, u.ID)
	if err != nil || got.PasswordHash != newHash {
		t.Fatalf("password not updated %+v", got)
	}
	if err := st.SetPassword(ctx, 99999, newHash); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing user: %v", err)
	}
}

func TestSetPasswordDeletesSessions(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	hash, err := auth.HashPassword("correct-horse")
	if err != nil {
		t.Fatal(err)
	}
	u, err := st.CreateUser(ctx, "bob", "Bob", hash, auth.RoleViewer)
	if err != nil {
		t.Fatal(err)
	}
	_, shash, err := auth.NewSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateSession(ctx, u.ID, shash, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UserBySessionHash(ctx, shash); err != nil {
		t.Fatal(err)
	}
	newHash, err := auth.HashPassword("new-secret-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetPassword(ctx, u.ID, newHash); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UserBySessionHash(ctx, shash); !errors.Is(err, ErrNotFound) {
		t.Fatalf("session should be gone: %v", err)
	}
}

func TestPurgeExpiredSessions(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	hash, err := auth.HashPassword("correct-horse")
	if err != nil {
		t.Fatal(err)
	}
	u, err := st.CreateUser(ctx, "bob", "Bob", hash, auth.RoleViewer)
	if err != nil {
		t.Fatal(err)
	}
	_, live, err := auth.NewSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	_, dead, err := auth.NewSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateSession(ctx, u.ID, live, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateSession(ctx, u.ID, dead, time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	n, err := st.PurgeExpiredSessions(ctx, time.Now())
	if err != nil || n != 1 {
		t.Fatalf("purged %d %v", n, err)
	}
	if _, err := st.UserBySessionHash(ctx, live); err != nil {
		t.Fatalf("live session: %v", err)
	}
	if _, err := st.UserBySessionHash(ctx, dead); !errors.Is(err, ErrNotFound) {
		t.Fatalf("dead session: %v", err)
	}
}

func TestUserLookupNotFound(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	if _, err := st.UserByID(ctx, 404); !errors.Is(err, ErrNotFound) {
		t.Fatalf("by id: %v", err)
	}
	if _, err := st.UserByUsername(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("by name: %v", err)
	}
}

func TestUpdateUserNotFound(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	if err := st.UpdateUser(ctx, 99999, auth.RoleViewer, true); !errors.Is(err, ErrNotFound) {
		t.Fatalf("update: %v", err)
	}
}

func TestCreateAPIKeyInvalidScope(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	hash, err := auth.HashPassword("correct-horse")
	if err != nil {
		t.Fatal(err)
	}
	u, err := st.CreateUser(ctx, "alice", "alice", hash, auth.RoleUploader)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateAPIKey(ctx, u.ID, "hash", "pre", auth.KeyScope("admin")); err == nil {
		t.Fatal("invalid scope")
	}
}
