package store

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/hrodrig/groot-share/internal/auth"
)

func TestNormalizeName(t *testing.T) {
	got, err := NormalizeName("  Juan   Negro  ")
	if err != nil || got != "Juan Negro" {
		t.Fatalf("got %q %v", got, err)
	}
	if _, err := NormalizeName("   "); !errors.Is(err, ErrNameRequired) {
		t.Fatalf("empty: %v", err)
	}
	if _, err := NormalizeName(strings.Repeat("á", MaxNameRunes+1)); !errors.Is(err, ErrNameTooLong) {
		t.Fatalf("long: %v", err)
	}
}

func TestTruncateName(t *testing.T) {
	if TruncateName("Juan Negro") != "Juan Negro" {
		t.Fatal(TruncateName("Juan Negro"))
	}
	long := "Juan Carlos Alberto Rodriguez Negro"
	got := TruncateName(long)
	if got == long || !strings.Contains(got, "...") || !strings.HasSuffix(got, "egro") {
		t.Fatalf("truncate %q", got)
	}
	if len([]rune(got)) != 30 {
		t.Fatalf("len %d %q", len([]rune(got)), got)
	}
	u := User{Username: "test", Name: long}
	if u.DisplayName() != got {
		t.Fatalf("display %q", u.DisplayName())
	}
	empty := User{Username: "test"}
	if empty.DisplayName() != "test" {
		t.Fatalf("fallback %q", empty.DisplayName())
	}
}

func TestNormalizeUsername(t *testing.T) {
	got, err := NormalizeUsername("  admin  ")
	if err != nil || got != "admin" {
		t.Fatalf("got %q %v", got, err)
	}
	if _, err := NormalizeUsername("   "); !errors.Is(err, ErrUsernameRequired) {
		t.Fatalf("empty: %v", err)
	}
	if _, err := NormalizeUsername(strings.Repeat("a", MaxUsernameRunes+1)); !errors.Is(err, ErrUsernameTooLong) {
		t.Fatalf("long: %v", err)
	}
}

func TestEnsureAdminCustomName(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	if err := st.EnsureAdmin(ctx, "root", "correct-horse", "Ada Lovelace"); err != nil {
		t.Fatal(err)
	}
	u, err := st.UserByUsername(ctx, "root")
	if err != nil || u.Name != "Ada Lovelace" {
		t.Fatalf("name %+v %v", u, err)
	}
}

func TestSetName(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	hash, err := auth.HashPassword("correct-horse")
	if err != nil {
		t.Fatal(err)
	}
	u, err := st.CreateUser(ctx, "bob", "Bob", hash, auth.RoleUploader)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetName(ctx, u.ID, "  Robert  Bob  "); err != nil {
		t.Fatal(err)
	}
	got, err := st.UserByID(ctx, u.ID)
	if err != nil || got.Name != "Robert Bob" {
		t.Fatalf("set %+v %v", got, err)
	}
	if err := st.SetName(ctx, u.ID, ""); !errors.Is(err, ErrNameRequired) {
		t.Fatalf("empty set: %v", err)
	}
}

func TestSetUsername(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	hash, err := auth.HashPassword("correct-horse")
	if err != nil {
		t.Fatal(err)
	}
	a, err := st.CreateUser(ctx, "alice", "Alice", hash, auth.RoleUploader)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateUser(ctx, "bob", "Bob", hash, auth.RoleViewer); err != nil {
		t.Fatal(err)
	}
	if err := st.SetUsername(ctx, a.ID, "  alice2  "); err != nil {
		t.Fatal(err)
	}
	got, err := st.UserByID(ctx, a.ID)
	if err != nil || got.Username != "alice2" {
		t.Fatalf("renamed %+v %v", got, err)
	}
	if err := st.SetUsername(ctx, a.ID, "bob"); err == nil {
		t.Fatal("expected unique")
	}
	if err := st.SetUsername(ctx, a.ID, ""); !errors.Is(err, ErrUsernameRequired) {
		t.Fatalf("empty: %v", err)
	}
}
