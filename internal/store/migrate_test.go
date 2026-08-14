package store

import (
	"context"
	"testing"

	"github.com/hrodrig/groot-share/internal/auth"
)

func TestMigrateAdminColumnToRole(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	if err := st.migrateLegacyV0(ctx); err != nil {
		t.Fatal(err)
	}
	hash, err := auth.HashPassword("correct-horse")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.ExecContext(ctx, `INSERT INTO users (username, password_hash, admin) VALUES ('root', ?, 1)`, hash); err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.ExecContext(ctx, `INSERT INTO users (username, password_hash, admin) VALUES ('bob', ?, 0)`, hash); err != nil {
		t.Fatal(err)
	}
	if err := st.migrate(); err != nil {
		t.Fatal(err)
	}
	root, err := st.UserByUsername(ctx, "root")
	if err != nil || root.Role != auth.RoleAdmin || root.Name != "root" {
		t.Fatalf("root %+v %v", root, err)
	}
	bob, err := st.UserByUsername(ctx, "bob")
	if err != nil || bob.Role != auth.RoleUploader {
		t.Fatalf("bob %+v %v", bob, err)
	}
}
