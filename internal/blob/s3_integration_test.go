package blob

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func fakeS3Server(t *testing.T, bucket string) (*httptest.Server, *S3) {
	t.Helper()
	store := map[string][]byte{}
	var mu sync.Mutex

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		trimmed := strings.TrimPrefix(r.URL.Path, "/")
		trimmed = strings.TrimPrefix(trimmed, bucket)
		trimmed = strings.TrimPrefix(trimmed, "/")
		switch r.Method {
		case http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			mu.Lock()
			store[trimmed] = body
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			if r.URL.Query().Get("list-type") == "2" {
				prefix := r.URL.Query().Get("prefix")
				mu.Lock()
				var b strings.Builder
				b.WriteString(`<?xml version="1.0"?><ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">`)
				for k, v := range store {
					if prefix != "" && !strings.HasPrefix(k, prefix) {
						continue
					}
					if strings.HasSuffix(k, "/") {
						continue
					}
					fmt.Fprintf(&b, `<Contents><Key>%s</Key><Size>%d</Size><ETag>"abc"</ETag><LastModified>2026-08-13T12:00:00.000Z</LastModified></Contents>`, k, len(v))
				}
				b.WriteString(`</ListBucketResult>`)
				mu.Unlock()
				w.Header().Set("Content-Type", "application/xml")
				_, _ = w.Write([]byte(b.String()))
				return
			}
			mu.Lock()
			body, ok := store[trimmed]
			mu.Unlock()
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("ETag", `"abc"`)
			_, _ = w.Write(body)
		case http.MethodHead:
			if trimmed == "" {
				w.WriteHeader(http.StatusOK)
				return
			}
			mu.Lock()
			_, ok := store[trimmed]
			mu.Unlock()
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Length", "5")
			w.WriteHeader(http.StatusOK)
		case http.MethodDelete:
			mu.Lock()
			delete(store, trimmed)
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	ctx := context.Background()
	cfg, err := awscfg.LoadDefaultConfig(ctx,
		awscfg.WithRegion("us-east-1"),
		awscfg.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("ak", "sk", "")),
	)
	if err != nil {
		t.Fatal(err)
	}
	api := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(srv.URL)
		o.UsePathStyle = true
	})
	return srv, &S3{api: api, bucket: bucket}
}

func TestS3CRUDViaFakeEndpoint(t *testing.T) {
	_, c := fakeS3Server(t, "lab")
	ctx := context.Background()

	if err := c.HeadBucket(ctx); err != nil {
		t.Fatalf("head bucket: %v", err)
	}
	key := "captures/demo.tar.gz"
	if err := c.Put(ctx, key, strings.NewReader("hello")); err != nil {
		t.Fatalf("put: %v", err)
	}
	obj, err := c.Head(ctx, key)
	if err != nil || obj.Key != key {
		t.Fatalf("head %+v %v", obj, err)
	}
	rc, got, err := c.Get(ctx, key)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	body, _ := io.ReadAll(rc)
	_ = rc.Close()
	if string(body) != "hello" || got.Key != key {
		t.Fatalf("get body %q obj %+v", body, got)
	}
	list, err := c.List(ctx, "captures/")
	if err != nil || len(list) != 1 || list[0].Key != key {
		t.Fatalf("list %+v %v", list, err)
	}
	if err := c.Delete(ctx, key); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := c.Head(ctx, key); err != ErrNotFound {
		t.Fatalf("head after delete: %v", err)
	}
	_, _, err = c.Get(ctx, "missing.tar.gz")
	if err != ErrNotFound {
		t.Fatalf("get missing: %v", err)
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, fmt.Errorf("read fail") }

func TestMemoryPutReadError(t *testing.T) {
	m := NewMemory()
	if err := m.Put(context.Background(), "k", errReader{}); err == nil {
		t.Fatal("expected read error")
	}
}

func TestMemoryListSkipsDirs(t *testing.T) {
	m := NewMemory()
	ctx := context.Background()
	if err := m.Put(ctx, "captures/a.tar.gz", strings.NewReader("a")); err != nil {
		t.Fatal(err)
	}
	if err := m.Put(ctx, "captures/dir/", strings.NewReader("")); err != nil {
		t.Fatal(err)
	}
	list, err := m.List(ctx, "captures/")
	if err != nil || len(list) != 1 {
		t.Fatalf("list %+v %v", list, err)
	}
}

func TestMemoryDeleteMissing(t *testing.T) {
	if err := NewMemory().Delete(context.Background(), "nope"); err != ErrNotFound {
		t.Fatalf("delete missing: %v", err)
	}
}
