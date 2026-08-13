package blob

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

// Memory is an in-process Store for unit tests (no live bucket).
type Memory struct {
	mu             sync.Mutex
	objects        map[string][]byte
	meta           map[string]Object
	FailPuts       bool
	FailHeadBucket bool
}

// NewMemory returns an empty fake bucket.
func NewMemory() *Memory {
	return &Memory{
		objects: map[string][]byte{},
		meta:    map[string]Object{},
	}
}

// Put stores r under key.
func (m *Memory) Put(_ context.Context, key string, r io.Reader) error {
	if m.FailPuts {
		return fmt.Errorf("forced put failure")
	}
	b, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}
	sum := sha256.Sum256(b)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects[key] = b
	m.meta[key] = Object{
		Key:          key,
		ETag:         hex.EncodeToString(sum[:]),
		Size:         int64(len(b)),
		LastModified: time.Now().UTC(),
	}
	return nil
}

type memBody struct {
	*bytes.Reader
}

func (memBody) Close() error { return nil }

// Get returns a seekable body.
func (m *Memory) Get(_ context.Context, key string) (io.ReadCloser, Object, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.objects[key]
	if !ok {
		return nil, Object{}, ErrNotFound
	}
	obj := m.meta[key]
	cp := append([]byte(nil), b...)
	return memBody{Reader: bytes.NewReader(cp)}, obj, nil
}

// List returns objects whose key has the prefix.
func (m *Memory) List(_ context.Context, prefix string) ([]Object, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Object, 0, len(m.meta))
	for k, obj := range m.meta {
		if prefix != "" && !strings.HasPrefix(k, prefix) {
			continue
		}
		if strings.HasSuffix(k, "/") {
			continue
		}
		out = append(out, obj)
	}
	return out, nil
}

// Head returns metadata or ErrNotFound.
func (m *Memory) Head(_ context.Context, key string) (Object, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	obj, ok := m.meta[key]
	if !ok {
		return Object{}, ErrNotFound
	}
	return obj, nil
}

// HeadBucket succeeds unless FailHeadBucket is set.
func (m *Memory) HeadBucket(_ context.Context) error {
	if m.FailHeadBucket {
		return fmt.Errorf("forced head bucket failure")
	}
	return nil
}

// Delete removes a key.
func (m *Memory) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.objects[key]; !ok {
		return ErrNotFound
	}
	delete(m.objects, key)
	delete(m.meta, key)
	return nil
}

// Touch sets LastModified (tests).
func (m *Memory) Touch(key string, t time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	obj := m.meta[key]
	obj.LastModified = t.UTC()
	m.meta[key] = obj
}
