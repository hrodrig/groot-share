// Package blob is the object-store boundary (S3-compatible or in-memory fake).
package blob

import (
	"context"
	"errors"
	"io"
	"time"
)

// ErrNotFound is a missing object or key.
var ErrNotFound = errors.New("not found")

// Object is listing/head metadata for one key.
type Object struct {
	Key          string
	ETag         string
	Size         int64
	LastModified time.Time
}

// Store is Put/Get/List against a bucket prefix. Implementations must not
// require the caller to buffer the whole body in RAM.
type Store interface {
	Put(ctx context.Context, key string, r io.Reader) error
	Get(ctx context.Context, key string) (io.ReadCloser, Object, error)
	List(ctx context.Context, prefix string) ([]Object, error)
	Head(ctx context.Context, key string) (Object, error)
	HeadBucket(ctx context.Context) error
	Delete(ctx context.Context, key string) error
}
