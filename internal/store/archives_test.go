package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIngestListDownload(t *testing.T) {
	st := archiveStore(t)
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
	st := archiveStore(t)
	if _, err := st.ArchiveByID(context.Background(), "../etc/passwd"); err != ErrNotFound {
		t.Fatalf("traversal %v", err)
	}
	if _, err := st.BlobPath(filepath.Join("..", "x")); err == nil {
		t.Fatal("bad id path")
	}
}

func TestIngestEmpty(t *testing.T) {
	st := archiveStore(t)
	_, err := st.Ingest(context.Background(), bytes.NewReader(nil), "x.tar.gz", 1)
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("err %v", err)
	}
}

func TestSanitizeKey(t *testing.T) {
	cases := map[string]string{
		"/tmp/../secret.tar.gz":              "secret.tar.gz",
		"":                                   "archive.tar.gz",
		"my file.tar.gz":                     "my-file.tar.gz",
		"foo@#bar$.tar.gz":                   "foobar.tar.gz",
		"kel-capture-AZKB335001.tar.gz":      "kel-capture-AZKB335001.tar.gz",
		" spaced  .tar.gz":                   "spaced-.tar.gz",
		"bad\u0000name.tar.gz":               "badname.tar.gz",
		"../../../etc/passwd":                "passwd",
		strings.Repeat("a", 180) + ".tar.gz": strings.Repeat("a", 180) + ".tar.gz",
	}
	for in, want := range cases {
		if got := sanitizeKey(in); got != want {
			t.Fatalf("sanitizeKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDeleteArchive(t *testing.T) {
	st := archiveStore(t)
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

// TestDeleteArchiveCleansDependents pins a fix for #39: deleting an archive
// must also drop its share_links and archive_pins rows (explicit cleanup, not
// FK cascade — vps-s3 archives may have no `archives` row). No orphans.
func TestDeleteArchiveCleansDependents(t *testing.T) {
	st := archiveStore(t)
	ctx := context.Background()
	a, err := st.Ingest(ctx, bytes.NewReader([]byte("cascade")), "cascade.tar.gz", 1)
	if err != nil {
		t.Fatal(err)
	}
	root, err := st.UserByUsername(ctx, "root")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.AddPin(ctx, root.ID, a); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateShareLink(ctx, a.ID, "hash-cascade", root.ID, "label", 0, time.Now().UTC().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteArchive(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	var pins, shares int
	if err := st.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM archive_pins WHERE archive_id = ?`, a.ID).Scan(&pins); err != nil {
		t.Fatal(err)
	}
	if err := st.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM share_links WHERE archive_id = ?`, a.ID).Scan(&shares); err != nil {
		t.Fatal(err)
	}
	if pins != 0 {
		t.Fatalf("orphan pins after delete: %d", pins)
	}
	if shares != 0 {
		t.Fatalf("orphan shares after delete: %d", shares)
	}
}

func TestIngestDuplicateSHA256(t *testing.T) {
	st := archiveStore(t)
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
	st := archiveStore(t)
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

func TestFindExistingSHA256FromTransit(t *testing.T) {
	st := archiveStore(t)
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
	st := archiveStore(t)
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
	st := archiveStore(t)
	ctx := context.Background()
	if _, err := st.StageWithID(ctx, "not-hex", bytes.NewReader([]byte("x")), "a.tar.gz", 1); err == nil {
		t.Fatal("invalid id")
	}
}

func TestCommitLocalPromotesStaging(t *testing.T) {
	st := archiveStore(t)
	ctx := context.Background()
	staged, err := st.Stage(ctx, bytes.NewReader([]byte("commit-me")), "promote.tar.gz", 1)
	if err != nil {
		t.Fatal(err)
	}
	a, err := st.CommitLocal(ctx, staged, "http")
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
	st := archiveStore(t)
	ctx := context.Background()
	staged := Staged{ID: "not-valid", Path: filepath.Join(st.StagingDir(), "x.partial")}
	if _, err := st.CommitLocal(ctx, staged, "http"); err == nil {
		t.Fatal("expected error")
	}
}

func TestCommitLocalMissingStagingFile(t *testing.T) {
	st := archiveStore(t)
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
	if _, err := st.CommitLocal(ctx, staged, "http"); err == nil {
		t.Fatal("expected rename error")
	}
}

func TestIngestDuplicateArchive(t *testing.T) {
	st := archiveStore(t)
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
	st := archiveStore(t)
	ctx := context.Background()
	staged1, err := st.Stage(ctx, bytes.NewReader([]byte("one")), "a.tar.gz", 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CommitLocal(ctx, staged1, "http"); err != nil {
		t.Fatal(err)
	}
	staged2, err := st.StageWithID(ctx, staged1.ID, bytes.NewReader([]byte("two")), "b.tar.gz", 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CommitLocal(ctx, staged2, "http"); err == nil {
		t.Fatal("expected duplicate insert error")
	}
}

func TestCommitLocalSFTPSource(t *testing.T) {
	st := archiveStore(t)
	ctx := context.Background()
	staged, err := st.Stage(ctx, bytes.NewReader([]byte("sftp-local")), "sftp.tar.gz", 0)
	if err != nil {
		t.Fatal(err)
	}
	a, err := st.CommitLocal(ctx, staged, "sftp")
	if err != nil || a.Source != "sftp" {
		t.Fatalf("%+v %v", a, err)
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, fmt.Errorf("read fail") }

func TestStageReadError(t *testing.T) {
	st := testStore(t)
	if _, err := st.Stage(context.Background(), errReader{}, "x.tar.gz", 1); err == nil {
		t.Fatal("expected stream error")
	}
}
