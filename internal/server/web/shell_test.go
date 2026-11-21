package web

import (
	"testing"

	"github.com/hrodrig/groot-share/internal/config"
	"github.com/hrodrig/groot-share/internal/server/core"
)

func TestPageShell(t *testing.T) {
	ui := New(core.NewCore(config.Config{}, nil, nil, "0.1.0-test", nil))
	data := ui.PageShell()
	if data == nil {
		t.Fatal("nil shell")
	}
}