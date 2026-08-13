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
