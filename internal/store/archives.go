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
	"unicode"
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

// DuplicateError is returned when an upload matches an existing archive by sha256.
type DuplicateError struct {
	Existing Archive
}

func (e *DuplicateError) Error() string {
	return "duplicate archive"
}

// Ingest streams r into VPS home and records metadata. Does not buffer r in RAM.
func (s *Store) Ingest(ctx context.Context, r io.Reader, key string, uploadedBy int64) (Archive, error) {
	st, err := s.Stage(ctx, r, key, uploadedBy)
	if err != nil {
		return Archive{}, err
	}
	if existing, err := s.FindExistingSHA256(ctx, st.SHA256); err == nil {
		_ = os.Remove(st.Path)
		return Archive{}, &DuplicateError{Existing: existing}
	} else if !errors.Is(err, ErrNotFound) {
		_ = os.Remove(st.Path)
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
		st.ID, st.Key, st.Size, st.SHA256, optionalUserID(st.UploadedBy), st.CreatedAt.Format(time.RFC3339),
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

// InsertArchiveMeta records metadata for objects stored outside VPS home (e.g. S3).
func (s *Store) InsertArchiveMeta(ctx context.Context, a Archive) error {
	if strings.TrimSpace(a.ID) == "" || strings.TrimSpace(a.SHA256) == "" {
		return fmt.Errorf("invalid archive metadata")
	}
	created := a.CreatedAt.UTC().Format(time.RFC3339)
	if a.CreatedAt.IsZero() {
		created = time.Now().UTC().Format(time.RFC3339)
	}
	source := a.Source
	if source == "" {
		source = "http"
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO archives (id, key, size, sha256, source, uploaded_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.Key, a.Size, a.SHA256, source, optionalUserID(a.UploadedBy), created,
	)
	if err != nil {
		return fmt.Errorf("insert archive meta: %w", err)
	}
	return nil
}

// ArchiveBySHA256 returns the newest archive with the given content hash.
func (s *Store) ArchiveBySHA256(ctx context.Context, sha256 string) (Archive, error) {
	if strings.TrimSpace(sha256) == "" {
		return Archive{}, ErrNotFound
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT id, key, size, sha256, source, COALESCE(uploaded_by, 0), created_at
		FROM archives WHERE sha256 = ?
		ORDER BY created_at DESC LIMIT 1`, sha256)
	a, err := scanArchive(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Archive{}, ErrNotFound
	}
	if err != nil {
		return Archive{}, fmt.Errorf("get archive by sha256: %w", err)
	}
	return a, nil
}

// FindExistingSHA256 returns an archive or in-flight transit row with the same hash.
func (s *Store) FindExistingSHA256(ctx context.Context, sha256 string) (Archive, error) {
	a, err := s.ArchiveBySHA256(ctx, sha256)
	if err == nil {
		return a, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return Archive{}, err
	}
	tr, err := s.TransitBySHA256(ctx, sha256)
	if errors.Is(err, ErrNotFound) {
		return Archive{}, ErrNotFound
	}
	if err != nil {
		return Archive{}, err
	}
	return archiveFromTransit(tr), nil
}

func archiveFromTransit(tr Transit) Archive {
	return Archive{
		ID:         tr.S3Key,
		Key:        tr.Key,
		Size:       tr.Size,
		SHA256:     tr.SHA256,
		Source:     "http",
		Storage:    "transit",
		UploadedBy: tr.UploadedBy,
		CreatedAt:  tr.CreatedAt,
	}
}

func sanitizeKey(key string) string {
	key = filepath.Base(strings.ReplaceAll(key, "\\", "/"))
	key = strings.TrimSpace(key)
	if key == "" || key == "." || key == "/" {
		return "archive.tar.gz"
	}
	var b strings.Builder
	b.Grow(len(key))
	for _, r := range key {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			b.WriteRune(r)
		case unicode.IsSpace(r):
			b.WriteRune('-')
		}
	}
	key = collapseDashes(strings.Trim(b.String(), "-._"))
	if key == "" {
		return "archive.tar.gz"
	}
	if len(key) > 200 {
		key = key[:200]
		key = strings.Trim(key, "-._")
	}
	if key == "" {
		return "archive.tar.gz"
	}
	return key
}

// SanitizeArchiveKey normalizes a client-provided capture filename for storage and display.
func SanitizeArchiveKey(key string) string {
	return sanitizeKey(key)
}

func collapseDashes(s string) string {
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return s
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

// DeleteArchive removes the sqlite row and the VPS home file.
func (s *Store) DeleteArchive(ctx context.Context, id string) error {
	if !validArchiveID(id) {
		return ErrNotFound
	}
	p, err := s.BlobPath(id)
	if err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM archives WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete archive: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove blob: %w", err)
	}
	return nil
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
