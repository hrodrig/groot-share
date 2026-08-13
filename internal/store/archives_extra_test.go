package store

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFindExistingSHA256FromTransit(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	staged, err := st.Stage(ctx, bytes.NewReader([]byte("in-flight")), "run.tar.gz", 1)
	if err != nil {
		t.Fatal(err)
	}
	s3key := "captures/2026/08/12/" + staged.ID + ".tar.gz"
	if err := st.SaveTransit(ctx, staged, s3key, "pending"); err != nil {
		t.Fatal(err)
	}
	got, err := st.FindExistingSHA256(ctx, staged.SHA256)
	if err != nil || got.Storage != "transit" || got.ID != s3key {
		t.Fatalf("transit dup %+v %v", got, err)
	}
	if _, err := st.ArchiveBySHA256(ctx, ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("empty sha256: %v", err)
	}
}

func TestInsertArchiveMeta(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	a := Archive{
		ID:        "captures/2026/08/12/abc.tar.gz",
		Key:       "abc.tar.gz",
		Size:      12,
		SHA256:    "deadbeef",
		Source:    "s3",
		CreatedAt: time.Now().UTC(),
	}
	if err := st.InsertArchiveMeta(ctx, a); err != nil {
		t.Fatal(err)
	}
	list, err := st.ListArchives(ctx)
	if err != nil || len(list) != 1 || list[0].Source != "s3" {
		t.Fatalf("list %+v %v", list, err)
	}
	if err := st.InsertArchiveMeta(ctx, Archive{}); err == nil {
		t.Fatal("invalid meta")
	}
}

func TestStageWithIDInvalid(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	if _, err := st.StageWithID(ctx, "not-hex", bytes.NewReader([]byte("x")), "a.tar.gz", 1); err == nil {
		t.Fatal("invalid id")
	}
}

func TestCommitLocalPromotesStaging(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	staged, err := st.Stage(ctx, bytes.NewReader([]byte("commit-me")), "promote.tar.gz", 2)
	if err != nil {
		t.Fatal(err)
	}
	a, err := st.CommitLocal(ctx, staged)
	if err != nil || a.Storage != "local" || a.Key != "promote.tar.gz" {
		t.Fatalf("%+v %v", a, err)
	}
	if _, err := st.ArchiveByID(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
}

func TestDuplicateErrorString(t *testing.T) {
	var err error = &DuplicateError{Existing: Archive{ID: "abc"}}
	if err.Error() != "duplicate archive" {
		t.Fatalf("%q", err.Error())
	}
}

func TestCommitLocalInvalidArchiveID(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	staged := Staged{ID: "not-valid", Path: filepath.Join(st.StagingDir(), "x.partial")}
	if _, err := st.CommitLocal(ctx, staged); err == nil {
		t.Fatal("expected error")
	}
}

func TestCommitLocalMissingStagingFile(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	id := strings.Repeat("a", 32)
	staged := Staged{
		ID:         id,
		Path:       filepath.Join(st.StagingDir(), "missing.partial"),
		Key:        "k.tar.gz",
		Size:       1,
		SHA256:     strings.Repeat("b", 64),
		UploadedBy: 1,
		CreatedAt:  time.Now().UTC(),
	}
	if _, err := st.CommitLocal(ctx, staged); err == nil {
		t.Fatal("expected rename error")
	}
}

func TestIngestDuplicateArchive(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	body := []byte("same-content")
	a1, err := st.Ingest(ctx, bytes.NewReader(body), "first.tar.gz", 1)
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.Ingest(ctx, bytes.NewReader(body), "second.tar.gz", 1)
	var dup *DuplicateError
	if !errors.As(err, &dup) || dup.Existing.ID != a1.ID {
		t.Fatalf("dup %+v %v", dup, err)
	}
}

func TestCommitLocalDuplicateInsert(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	staged1, err := st.Stage(ctx, bytes.NewReader([]byte("one")), "a.tar.gz", 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CommitLocal(ctx, staged1); err != nil {
		t.Fatal(err)
	}
	staged2, err := st.StageWithID(ctx, staged1.ID, bytes.NewReader([]byte("two")), "b.tar.gz", 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CommitLocal(ctx, staged2); err == nil {
		t.Fatal("expected duplicate insert error")
	}
}
