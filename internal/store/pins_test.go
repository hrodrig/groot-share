package store

import (
	"context"
	"testing"

	"github.com/hrodrig/groot-share/internal/auth"
)

func TestAddPinIdempotent(t *testing.T) {
	st := archiveStore(t)
	ctx := context.Background()

	root, err := st.UserByUsername(ctx, "root")
	if err != nil {
		t.Fatal(err)
	}
	a := Archive{ID: "abc123", Key: "capture-prod.tar.gz", Size: 1024}
	if err := st.AddPin(ctx, root.ID, a); err != nil {
		t.Fatal(err)
	}
	// Second add with the same id must not error and must not duplicate.
	if err := st.AddPin(ctx, root.ID, a); err != nil {
		t.Fatal(err)
	}
	pins, err := st.ListPins(ctx, root.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(pins) != 1 {
		t.Fatalf("want 1 pin, got %d", len(pins))
	}
	if pins[0].ArchiveID != "abc123" || pins[0].ArchiveKey != "capture-prod.tar.gz" || pins[0].Size != 1024 {
		t.Fatalf("pin snapshot mismatch: %+v", pins[0])
	}
}

func TestAddPinSnapshotsKeyAndSize(t *testing.T) {
	st := archiveStore(t)
	ctx := context.Background()
	root, err := st.UserByUsername(ctx, "root")
	if err != nil {
		t.Fatal(err)
	}
	a := Archive{ID: "deadbeef", Key: "v1-prod-cluster-1-20260821.tar.gz", Size: 5 << 20}
	if err := st.AddPin(ctx, root.ID, a); err != nil {
		t.Fatal(err)
	}
	// Mutating the caller's struct after the pin must not change what the
	// store has snapshotted.
	a.Key = "different.tar.gz"
	a.Size = 999
	pins, err := st.ListPins(ctx, root.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(pins) != 1 {
		t.Fatalf("want 1 pin, got %d", len(pins))
	}
	if pins[0].ArchiveKey != "v1-prod-cluster-1-20260821.tar.gz" || pins[0].Size != 5<<20 {
		t.Fatalf("pin snapshot not held: %+v", pins[0])
	}
}

func TestRemovePinIdempotent(t *testing.T) {
	st := archiveStore(t)
	ctx := context.Background()
	root, err := st.UserByUsername(ctx, "root")
	if err != nil {
		t.Fatal(err)
	}
	a := Archive{ID: "id1", Key: "k1", Size: 1}
	if err := st.AddPin(ctx, root.ID, a); err != nil {
		t.Fatal(err)
	}

	removed, err := st.RemovePin(ctx, root.ID, "id1")
	if err != nil {
		t.Fatal(err)
	}
	if !removed {
		t.Fatal("first RemovePin must report removed=true")
	}
	removed, err = st.RemovePin(ctx, root.ID, "id1")
	if err != nil {
		t.Fatal(err)
	}
	if removed {
		t.Fatal("second RemovePin must report removed=false (idempotent)")
	}
}

func TestPinsArePerUser(t *testing.T) {
	st := archiveStore(t)
	ctx := context.Background()
	root, err := st.UserByUsername(ctx, "root")
	if err != nil {
		t.Fatal(err)
	}
	hash, err := auth.HashPassword("correct-horse")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateUser(ctx, "alice", "alice", hash, auth.RoleUploader); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateUser(ctx, "bob", "bob", hash, auth.RoleUploader); err != nil {
		t.Fatal(err)
	}
	alice, err := st.UserByUsername(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	bob, err := st.UserByUsername(ctx, "bob")
	if err != nil {
		t.Fatal(err)
	}

	a := Archive{ID: "shared", Key: "shared.tar.gz", Size: 42}
	if err := st.AddPin(ctx, alice.ID, a); err != nil {
		t.Fatal(err)
	}
	if err := st.AddPin(ctx, bob.ID, a); err != nil {
		t.Fatal(err)
	}
	// root did not pin → empty list
	if pins, _ := st.ListPins(ctx, root.ID, 0); len(pins) != 0 {
		t.Fatalf("root must have 0 pins, got %d", len(pins))
	}
	if pins, _ := st.ListPins(ctx, alice.ID, 0); len(pins) != 1 {
		t.Fatalf("alice must have 1 pin, got %d", len(pins))
	}
	if pins, _ := st.ListPins(ctx, bob.ID, 0); len(pins) != 1 {
		t.Fatalf("bob must have 1 pin, got %d", len(pins))
	}
	// Removing alice's pin must not touch bob's
	if _, err := st.RemovePin(ctx, alice.ID, "shared"); err != nil {
		t.Fatal(err)
	}
	if pins, _ := st.ListPins(ctx, alice.ID, 0); len(pins) != 0 {
		t.Fatalf("alice must have 0 pins after unpin, got %d", len(pins))
	}
	if pins, _ := st.ListPins(ctx, bob.ID, 0); len(pins) != 1 {
		t.Fatalf("bob must still have 1 pin, got %d", len(pins))
	}
}

func TestPinsCascadeOnUserDelete(t *testing.T) {
	st := archiveStore(t)
	ctx := context.Background()
	hash, err := auth.HashPassword("correct-horse")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateUser(ctx, "carol", "carol", hash, auth.RoleUploader); err != nil {
		t.Fatal(err)
	}
	carol, err := st.UserByUsername(ctx, "carol")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.AddPin(ctx, carol.ID, Archive{ID: "x", Key: "x.tar.gz", Size: 7}); err != nil {
		t.Fatal(err)
	}
	if pins, _ := st.ListPins(ctx, carol.ID, 0); len(pins) != 1 {
		t.Fatalf("carol must have 1 pin pre-delete, got %d", len(pins))
	}
	// Direct DELETE exercises the FK ON DELETE CASCADE contract; we do not
	// have a HardDeleteUser helper and that is intentional — user deletion
	// in the app is soft (`active = 0`), so this test uses raw SQL.
	if _, err := st.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, carol.ID); err != nil {
		t.Fatal(err)
	}
	// After user delete, their pin rows must be gone (FK ON DELETE CASCADE).
	if pins, _ := st.ListPins(ctx, carol.ID, 0); len(pins) != 0 {
		t.Fatalf("carol's pins must be gone after delete, got %d", len(pins))
	}
	// Spot-check: no orphan pin rows remain.
	var n int
	if err := st.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM archive_pins WHERE user_id = ?`, carol.ID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("orphan pin rows: %d", n)
	}
}

func TestListPinsLimit(t *testing.T) {
	st := archiveStore(t)
	ctx := context.Background()
	root, err := st.UserByUsername(ctx, "root")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if err := st.AddPin(ctx, root.ID, Archive{ID: "id-" + string(rune('a'+i)), Key: "k.tar.gz", Size: int64(i)}); err != nil {
			t.Fatal(err)
		}
	}
	pins, err := st.ListPins(ctx, root.ID, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(pins) != 3 {
		t.Fatalf("limit=3 should return 3, got %d", len(pins))
	}
}

func TestAddPinValidation(t *testing.T) {
	st := archiveStore(t)
	ctx := context.Background()
	if err := st.AddPin(ctx, 0, Archive{ID: "x", Key: "k", Size: 1}); err == nil {
		t.Fatal("AddPin with userID=0 must error")
	}
	root, _ := st.UserByUsername(ctx, "root")
	if err := st.AddPin(ctx, root.ID, Archive{ID: "", Key: "k", Size: 1}); err == nil {
		t.Fatal("AddPin with empty archive id must error")
	}
	if _, err := st.RemovePin(ctx, 0, "x"); err == nil {
		t.Fatal("RemovePin with userID=0 must error")
	}
	if _, err := st.RemovePin(ctx, root.ID, ""); err == nil {
		t.Fatal("RemovePin with empty archive id must error")
	}
	if _, err := st.ListPins(ctx, 0, 0); err == nil {
		t.Fatal("ListPins with userID=0 must error")
	}
}
