// Package store opens the gfs SQLite file (metadata home). Phase 2: ping only.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // register pure-Go sqlite driver (CGO=0)
)

// Store is a SQLite handle (users, sessions, api_keys).
type Store struct {
	db *sql.DB
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
	st := &Store{db: db}
	if err := st.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return st, nil
}

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

// Close releases the database handle.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}
