package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteJSONError(t *testing.T) {
	rr := httptest.NewRecorder()
	WriteJSONError(rr, http.StatusTeapot, "nope")
	if rr.Code != http.StatusTeapot {
		t.Fatalf("status %d", rr.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != "nope" {
		t.Fatalf("body %#v", body)
	}
}

func TestWantsJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept", "application/json")
	if !WantsJSON(req) {
		t.Fatal("expected json accept")
	}
	req.Header.Del("Accept")
	if WantsJSON(req) {
		t.Fatal("expected no json accept")
	}
}
