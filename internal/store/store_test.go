package store

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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
	info, err := os.Stat(filepath.Join(dir, "gfs.db"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("db mode %o want 0600", info.Mode().Perm())
	}
	var fk, timeout, journal string
	if err := st.db.QueryRow(`PRAGMA foreign_keys`).Scan(&fk); err != nil || fk != "1" {
		t.Fatalf("foreign_keys=%q %v", fk, err)
	}
	if err := st.db.QueryRow(`PRAGMA busy_timeout`).Scan(&timeout); err != nil || timeout != "5000" {
		t.Fatalf("busy_timeout=%q %v", timeout, err)
	}
	if err := st.db.QueryRow(`PRAGMA journal_mode`).Scan(&journal); err != nil || strings.ToLower(journal) != "wal" {
		t.Fatalf("journal_mode=%q %v", journal, err)
	}
	_, err = st.db.Exec(`INSERT INTO sessions (user_id, token_hash, expires_at) VALUES (99999, 'deadbeef', '2026-01-01T00:00:00Z')`)
	if err == nil {
		t.Fatal("expected foreign key rejection")
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
