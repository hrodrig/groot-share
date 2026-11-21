package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hrodrig/groot-share/internal/config"
	"github.com/hrodrig/groot-share/internal/server/core"
)

func TestHandleMeUnauthorized(t *testing.T) {
	c := core.NewCore(config.Config{}, nil, nil, "test", nil)
	api := New(c, http.NotFound, nil)
	rr := httptest.NewRecorder()
	api.HandleMe(rr, httptest.NewRequest(http.MethodGet, "/v1/me", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("code %d", rr.Code)
	}
}
