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

// Archive is a listed groot tarball (VPS home metadata or a prefix object).
type Archive struct {
	ID         string
	Key        string
	Size       int64
	SHA256     string
	Source     string
	Storage    string // local | s3 | transit; empty on sqlite vps rows
	UploadedBy int64
	CreatedAt  time.Time
}

// Staged is an in-flight file under staging/ (not listed).
type Staged struct {
	ID         string
	Key        string
	Path       string
	SHA256     string
	Size       int64
	UploadedBy int64
	CreatedAt  time.Time
}

// Transit is a staged object waiting for a successful bucket copy.
type Transit struct {
	Staged
	S3Key     string
	LastError string
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

// NewArchiveID returns 32 hex chars.
func NewArchiveID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// Ingest streams r into VPS home and records metadata. Does not buffer r in RAM.
func (s *Store) Ingest(ctx context.Context, r io.Reader, key string, uploadedBy int64) (Archive, error) {
	st, err := s.Stage(ctx, r, key, uploadedBy)
	if err != nil {
		return Archive{}, err
	}
	return s.CommitLocal(ctx, st)
}

// Stage writes r to staging/{id}.partial. Caller must CommitLocal or SaveTransit.
func (s *Store) Stage(ctx context.Context, r io.Reader, key string, uploadedBy int64) (Staged, error) {
	id, err := NewArchiveID()
	if err != nil {
		return Staged{}, fmt.Errorf("id: %w", err)
	}
	return s.StageWithID(ctx, id, r, key, uploadedBy)
}

// StageWithID writes staging using a caller-chosen 32-hex id.
func (s *Store) StageWithID(_ context.Context, id string, r io.Reader, key string, uploadedBy int64) (Staged, error) {
	if !validArchiveID(id) {
		return Staged{}, fmt.Errorf("invalid archive id")
	}
	key = sanitizeKey(key)
	stage := filepath.Join(s.StagingDir(), id+".partial")
	f, err := os.OpenFile(stage, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o640)
	if err != nil {
		return Staged{}, fmt.Errorf("staging: %w", err)
	}
	h := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(f, h), r)
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(stage)
		return Staged{}, fmt.Errorf("stream: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(stage)
		return Staged{}, fmt.Errorf("close staging: %w", closeErr)
	}
	if n == 0 {
		_ = os.Remove(stage)
		return Staged{}, fmt.Errorf("empty upload")
	}
	return Staged{
		ID:         id,
		Key:        key,
		Path:       stage,
		SHA256:     hex.EncodeToString(h.Sum(nil)),
		Size:       n,
		UploadedBy: uploadedBy,
		CreatedAt:  time.Now().UTC(),
	}, nil
}

// CommitLocal promotes staging into home and inserts an archives row.
func (s *Store) CommitLocal(ctx context.Context, st Staged) (Archive, error) {
	home, err := s.BlobPath(st.ID)
	if err != nil {
		_ = os.Remove(st.Path)
		return Archive{}, err
	}
	if err := os.Rename(st.Path, home); err != nil {
		_ = os.Remove(st.Path)
		return Archive{}, fmt.Errorf("commit blob: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO archives (id, key, size, sha256, source, uploaded_by, created_at)
		VALUES (?, ?, ?, ?, 'http', ?, ?)`,
		st.ID, st.Key, st.Size, st.SHA256, st.UploadedBy, st.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		_ = os.Remove(home)
		return Archive{}, fmt.Errorf("insert archive: %w", err)
	}
	return Archive{
		ID:         st.ID,
		Key:        st.Key,
		Size:       st.Size,
		SHA256:     st.SHA256,
		Source:     "http",
		Storage:    "local",
		UploadedBy: st.UploadedBy,
		CreatedAt:  st.CreatedAt,
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
