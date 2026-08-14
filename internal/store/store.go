// Package store opens the gfs SQLite file (metadata home). Phase 2: ping only.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite" // register pure-Go sqlite driver (CGO=0)
)

// Store is a SQLite handle plus local blob directories.
type Store struct {
	db  *sql.DB
	dir string
}

// Open creates dataDir if needed and opens gfs.db with the pure-Go driver.
func Open(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		return nil, fmt.Errorf("mkdir data dir: %w", err)
	}
	dsn := filepath.Join(dataDir, "gfs.db")
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(time.Hour)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	st := &Store{db: db, dir: dataDir}
	if err := st.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	if err := os.MkdirAll(st.HomeDir(), 0o750); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("mkdir home: %w", err)
	}
	if err := os.MkdirAll(st.StagingDir(), 0o750); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("mkdir staging: %w", err)
	}
	return st, nil
}

// HomeDir is the VPS blob home (durable on topology vps).
func (s *Store) HomeDir() string { return filepath.Join(s.dir, "home") }

// StagingDir is in-flight ingest (not listed).
func (s *Store) StagingDir() string { return filepath.Join(s.dir, "staging") }

// Ping reports whether SQLite still answers.
func (s *Store) Ping(ctx context.Context) bool {
	if s == nil || s.db == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := s.db.PingContext(ctx); err != nil {
		return false
	}
	var one int
	if err := s.db.QueryRowContext(ctx, "SELECT 1").Scan(&one); err != nil {
		return false
	}
	return one == 1
}

func parseDBTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05Z"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// Close releases the database handle.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}
