package store

import (
	"bytes"
	"context"
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
