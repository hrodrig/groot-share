package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Archive is a listed groot tarball (VPS home metadata).
type Archive struct {
	ID         string
	Key        string
	Size       int64
	SHA256     string
	Source     string
	UploadedBy int64
	CreatedAt  time.Time
}

// BlobPath is the home file for id. id must be 32 hex chars.
func (s *Store) BlobPath(id string) (string, error) {
	if !validArchiveID(id) {
		return "", fmt.Errorf("invalid archive id")
	}
	return filepath.Join(s.HomeDir(), id+".tar.gz"), nil
}

func validArchiveID(id string) bool {
	if len(id) != 32 {
		return false
	}
	for _, c := range id {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func newArchiveID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// Ingest streams r into home and records metadata. Does not buffer r in RAM.
func (s *Store) Ingest(ctx context.Context, r io.Reader, key string, uploadedBy int64) (Archive, error) {
	id, err := newArchiveID()
	if err != nil {
		return Archive{}, fmt.Errorf("id: %w", err)
	}
	key = sanitizeKey(key)
	stage := filepath.Join(s.StagingDir(), id+".partial")
	f, err := os.OpenFile(stage, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o640)
	if err != nil {
		return Archive{}, fmt.Errorf("staging: %w", err)
	}
	h := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(f, h), r)
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(stage)
		return Archive{}, fmt.Errorf("stream: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(stage)
		return Archive{}, fmt.Errorf("close staging: %w", closeErr)
	}
	if n == 0 {
		_ = os.Remove(stage)
		return Archive{}, fmt.Errorf("empty upload")
	}
	home, err := s.BlobPath(id)
	if err != nil {
		_ = os.Remove(stage)
		return Archive{}, err
	}
	if err := os.Rename(stage, home); err != nil {
		_ = os.Remove(stage)
		return Archive{}, fmt.Errorf("commit blob: %w", err)
	}
	sum := hex.EncodeToString(h.Sum(nil))
	now := time.Now().UTC()
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO archives (id, key, size, sha256, source, uploaded_by, created_at)
		VALUES (?, ?, ?, ?, 'http', ?, ?)`,
		id, key, n, sum, uploadedBy, now.Format(time.RFC3339),
	)
	if err != nil {
		_ = os.Remove(home)
		return Archive{}, fmt.Errorf("insert archive: %w", err)
	}
	return Archive{
		ID: id, Key: key, Size: n, SHA256: sum, Source: "http",
		UploadedBy: uploadedBy, CreatedAt: now,
	}, nil
}

func sanitizeKey(key string) string {
	key = filepath.Base(strings.ReplaceAll(key, "\\", "/"))
	key = strings.TrimSpace(key)
	if key == "" || key == "." || key == "/" {
		return "archive.tar.gz"
	}
	return key
}

// ListArchives returns newest first.
func (s *Store) ListArchives(ctx context.Context) ([]Archive, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, key, size, sha256, source, COALESCE(uploaded_by, 0), created_at
		FROM archives ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list archives: %w", err)
	}
	defer rows.Close()
	var out []Archive
	for rows.Next() {
		a, err := scanArchive(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ArchiveByID loads metadata.
func (s *Store) ArchiveByID(ctx context.Context, id string) (Archive, error) {
	if !validArchiveID(id) {
		return Archive{}, ErrNotFound
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT id, key, size, sha256, source, COALESCE(uploaded_by, 0), created_at
		FROM archives WHERE id = ?`, id)
	a, err := scanArchive(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Archive{}, ErrNotFound
	}
	if err != nil {
		return Archive{}, fmt.Errorf("get archive: %w", err)
	}
	return a, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanArchive(sc rowScanner) (Archive, error) {
	var a Archive
	var created string
	if err := sc.Scan(&a.ID, &a.Key, &a.Size, &a.SHA256, &a.Source, &a.UploadedBy, &created); err != nil {
		return Archive{}, err
	}
	if t, err := time.Parse(time.RFC3339, created); err == nil {
		a.CreatedAt = t
	}
	return a, nil
}
