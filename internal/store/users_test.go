package store

import (
	"context"
	"testing"

	"github.com/hrodrig/groot-share/internal/auth"
)

func TestGuardLastAdmin(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	hash, err := auth.HashPassword("correct-horse")
	if err != nil {
		t.Fatal(err)
	}
	root, err := st.CreateUser(ctx, "root", "root", hash, auth.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.GuardLastAdmin(ctx, root.ID, auth.RoleAdmin, false); err != ErrLastAdmin {
		t.Fatalf("want last admin, got %v", err)
	}
	if err := st.GuardLastAdmin(ctx, root.ID, auth.RoleUploader, true); err != ErrLastAdmin {
		t.Fatalf("demote last admin: %v", err)
	}
	other, err := st.CreateUser(ctx, "backup", "backup", hash, auth.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.GuardLastAdmin(ctx, root.ID, auth.RoleUploader, true); err != nil {
		t.Fatalf("two admins demote one: %v", err)
	}
	if err := st.UpdateUser(ctx, root.ID, auth.RoleUploader, true); err != nil {
		t.Fatal(err)
	}
	if err := st.GuardLastAdmin(ctx, other.ID, auth.RoleUploader, false); err != ErrLastAdmin {
		t.Fatalf("last remaining admin: %v", err)
	}
}

func TestListUsers(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	hash, err := auth.HashPassword("correct-horse")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateUser(ctx, "zebra", "zebra", hash, auth.RoleViewer); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateUser(ctx, "alpha", "alpha", hash, auth.RoleUploader); err != nil {
		t.Fatal(err)
	}
	items, err := st.ListUsers(ctx)
	if err != nil || len(items) != 2 {
		t.Fatalf("list len=%d err=%v", len(items), err)
	}
	if items[0].Username != "alpha" || items[1].Username != "zebra" {
		t.Fatalf("order %q %q", items[0].Username, items[1].Username)
	}
}

func TestRemoveUser(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	hash, err := auth.HashPassword("correct-horse")
	if err != nil {
		t.Fatal(err)
	}
	root, err := st.CreateUser(ctx, "root", "root", hash, auth.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.RemoveUser(ctx, root.ID); err != ErrLastAdmin {
		t.Fatalf("last admin remove: %v", err)
	}
	bob, err := st.CreateUser(ctx, "bob", "bob", hash, auth.RoleUploader)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.RemoveUser(ctx, bob.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UserByID(ctx, bob.ID); err != ErrNotFound {
		t.Fatalf("bob still there: %v", err)
	}
}
