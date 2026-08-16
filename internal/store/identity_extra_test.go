package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hrodrig/groot-share/internal/auth"
)

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
