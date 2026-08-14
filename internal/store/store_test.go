package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenPingClose(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	if !st.Ping(context.Background()) {
		t.Fatal("ping")
	}
	if _, err := os.Stat(filepath.Join(dir, "gfs.db")); err != nil {
		t.Fatalf("db file: %v", err)
	}
}

func TestParseDBTime(t *testing.T) {
	if parseDBTime("2026-08-14 00:46:13").IsZero() {
		t.Fatal("sqlite datetime")
	}
	if parseDBTime("2026-08-14T00:46:13Z").IsZero() {
		t.Fatal("rfc3339")
	}
	if !parseDBTime("").IsZero() {
		t.Fatal("empty")
	}
}

func TestPingNil(t *testing.T) {
	var st *Store
	if st.Ping(context.Background()) {
		t.Fatal("nil store should not ping")
	}
}

func TestOpenNotWritable(t *testing.T) {
	dir := t.TempDir()
	blocked := filepath.Join(dir, "blocked")
	if err := os.WriteFile(blocked, []byte("nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(blocked); err == nil {
		t.Fatal("expected error when data dir is a file")
	}
}
