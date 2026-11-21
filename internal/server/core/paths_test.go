package core

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDownloadID(t *testing.T) {
	t.Parallel()
	cases := []struct {
		path string
		want string
	}{
		{"/v1/archives/abc/file", "abc"},
		{"/v1/archives/cluster/key.tar.gz/file", "cluster/key.tar.gz"},
		{"/v1/archives/abc/", "abc"},
	}
	for _, tc := range cases {
		mux := http.NewServeMux()
		var got string
		mux.HandleFunc("GET /v1/archives/{id...}", func(w http.ResponseWriter, r *http.Request) {
			got = DownloadID(r)
		})
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		mux.ServeHTTP(httptest.NewRecorder(), req)
		if got != tc.want {
			t.Fatalf("%s: got %q want %q", tc.path, got, tc.want)
		}
	}
}

func TestDeleteID(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	var got string
	mux.HandleFunc("DELETE /v1/archives/{id...}", func(w http.ResponseWriter, r *http.Request) {
		got = DeleteID(r)
	})
	req := httptest.NewRequest(http.MethodDelete, "/v1/archives/cluster/a.tar.gz", nil)
	mux.ServeHTTP(httptest.NewRecorder(), req)
	if got != "cluster/a.tar.gz" {
		t.Fatalf("got %q", got)
	}
}
