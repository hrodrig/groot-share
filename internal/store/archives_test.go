package store

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIngestListDownload(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	payload := []byte("not-really-gzip-but-ok")
	a, err := st.Ingest(ctx, bytes.NewReader(payload), "run.tar.gz", 1)
	if err != nil {
		t.Fatal(err)
	}
	if a.Size != int64(len(payload)) || a.Source != "http" || len(a.ID) != 32 {
		t.Fatalf("%+v", a)
	}
	p, err := st.BlobPath(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil || !bytes.Equal(b, payload) {
		t.Fatalf("blob %v %q", err, b)
	}
	list, err := st.ListArchives(ctx)
	if err != nil || len(list) != 1 || list[0].ID != a.ID {
		t.Fatalf("list %+v %v", list, err)
	}
	got, err := st.ArchiveByID(ctx, a.ID)
	if err != nil || got.Key != "run.tar.gz" {
		t.Fatalf("get %+v %v", got, err)
	}
}

func TestArchiveIDRejectsTraversal(t *testing.T) {
	st := testStore(t)
	if _, err := st.ArchiveByID(context.Background(), "../etc/passwd"); err != ErrNotFound {
		t.Fatalf("traversal %v", err)
	}
	if _, err := st.BlobPath(filepath.Join("..", "x")); err == nil {
		t.Fatal("bad id path")
	}
}

func TestIngestEmpty(t *testing.T) {
	st := testStore(t)
	_, err := st.Ingest(context.Background(), bytes.NewReader(nil), "x.tar.gz", 1)
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("err %v", err)
	}
}

func TestSanitizeKey(t *testing.T) {
	if sanitizeKey("/tmp/../secret.tar.gz") != "secret.tar.gz" {
		t.Fatal(sanitizeKey("/tmp/../secret.tar.gz"))
	}
	if sanitizeKey("") != "archive.tar.gz" {
		t.Fatal(sanitizeKey(""))
	}
}

func TestDeleteArchive(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	a, err := st.Ingest(ctx, bytes.NewReader([]byte("bye")), "gone.tar.gz", 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteArchive(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ArchiveByID(ctx, a.ID); err != ErrNotFound {
		t.Fatalf("want not found %v", err)
	}
	if _, err := os.Stat(filepath.Join(st.HomeDir(), a.ID+".tar.gz")); !os.IsNotExist(err) {
		t.Fatalf("blob still there: %v", err)
	}
}

func TestIngestDuplicateSHA256(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	payload := []byte("same-bytes")
	first, err := st.Ingest(ctx, bytes.NewReader(payload), "one.tar.gz", 1)
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.Ingest(ctx, bytes.NewReader(payload), "two.tar.gz", 1)
	var dup *DuplicateError
	if !errors.As(err, &dup) {
		t.Fatalf("want duplicate, got %v", err)
	}
	if dup.Existing.ID != first.ID || dup.Existing.SHA256 != first.SHA256 {
		t.Fatalf("existing %+v first %+v", dup.Existing, first)
	}
	list, err := st.ListArchives(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("list %+v %v", list, err)
	}
}

func TestIngestDuplicateAfterDelete(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	payload := []byte("reupload-bytes")
	a, err := st.Ingest(ctx, bytes.NewReader(payload), "once.tar.gz", 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteArchive(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	b, err := st.Ingest(ctx, bytes.NewReader(payload), "again.tar.gz", 1)
	if err != nil {
		t.Fatalf("reupload after delete: %v", err)
	}
	if b.ID == a.ID {
		t.Fatalf("expected new id, got same %s", b.ID)
	}
}
