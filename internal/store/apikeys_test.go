package store

import (
	"context"
	"testing"

	"github.com/hrodrig/groot-share/internal/auth"
)

func TestAPIKeyByIDAndDelete(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	hash, err := auth.HashPassword("pw-secret-1")
	if err != nil {
		t.Fatal(err)
	}
	u, err := st.CreateUser(ctx, "u", hash, auth.RoleUploader)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateAPIKey(ctx, u.ID, "h1", "pfx1", auth.KeyScopeUpload); err != nil {
		t.Fatal(err)
	}
	keys, err := st.ListAPIKeysByUser(ctx, u.ID)
	if err != nil || len(keys) != 1 {
		t.Fatalf("list %v %d", err, len(keys))
	}
	got, err := st.APIKeyByID(ctx, keys[0].ID)
	if err != nil || got.Prefix != "pfx1" {
		t.Fatalf("by id %v %+v", err, got)
	}
	if _, err := st.APIKeyByID(ctx, 99999); err != ErrNotFound {
		t.Fatalf("missing id: %v", err)
	}
	if err := st.DeleteAPIKey(ctx, keys[0].ID, u.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteAPIKey(ctx, keys[0].ID, u.ID); err != ErrNotFound {
		t.Fatalf("double delete: %v", err)
	}
}
