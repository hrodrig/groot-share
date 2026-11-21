package core

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/hrodrig/groot-share/internal/config"
	"github.com/hrodrig/groot-share/internal/store"
)

func TestRecordAuditNoStore(t *testing.T) {
	c := NewCore(config.Config{}, nil, nil, "", nil)
	r := httptest.NewRequest("GET", "/", nil)
	c.RecordAudit(r, "download", store.Archive{ID: "1", Key: "a.tar.gz"})
}

func TestInsertAuditTestHook(t *testing.T) {
	var called bool
	c := NewCore(config.Config{}, &store.Store{}, nil, "", nil)
	c.AuditInsert = func(ctx context.Context, ev store.Audit) error {
		called = true
		return nil
	}
	if err := c.InsertAudit(context.Background(), store.Audit{Action: "x"}); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("expected hook")
	}
}