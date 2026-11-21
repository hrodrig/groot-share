package core

import (
	"context"
	"testing"

	"github.com/hrodrig/groot-share/internal/config"
	"github.com/hrodrig/groot-share/internal/store"
)

func TestListItemsLocalEmpty(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	c := NewCore(config.Config{Topology: config.TopologyVPS}, st, nil, "test", nil)
	items, err := c.ListItems(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("expected empty, got %d", len(items))
	}
}

func TestUseBucket(t *testing.T) {
	c := NewCore(config.Config{Topology: config.TopologyVPS}, nil, nil, "", nil)
	if c.UseBucket() {
		t.Fatal("vps should not use bucket")
	}
	c.Cfg.Topology = config.TopologyVPSS3
	c.Blobs = nil
	if c.UseBucket() {
		t.Fatal("nil blobs should not use bucket")
	}
}
