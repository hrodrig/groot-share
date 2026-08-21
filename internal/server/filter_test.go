package server

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestParseFilterEmpty(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	f := ParseFilter(req)
	if !f.IsZero() {
		t.Fatalf("empty request should yield zero filter: %+v", f)
	}
}

func TestParseFilterUnknownWindowTreatedAsAll(t *testing.T) {
	req := httptest.NewRequest("GET", "/?window=99h", nil)
	f := ParseFilter(req)
	if !f.Since.IsZero() {
		t.Fatalf("unknown window must not set Since: %v", f.Since)
	}
}

func TestParseFilterWindowSince(t *testing.T) {
	cases := []struct {
		window string
		want   time.Duration
	}{
		{"24h", 24 * time.Hour},
		{"7d", 7 * 24 * time.Hour},
		{"30d", 30 * 24 * time.Hour},
	}
	now := time.Date(2026, 8, 21, 19, 0, 0, 0, time.UTC)
	for _, c := range cases {
		t.Run(c.window, func(t *testing.T) {
			got := windowSince(c.window, now)
			want := now.Add(-c.want)
			if !got.Equal(want) {
				t.Fatalf("window=%s: want %v, got %v", c.window, want, got)
			}
		})
	}
}

func TestParseFilterAllows(t *testing.T) {
	cases := []struct {
		name    string
		url     string
		cluster string
		query   string
		source  string
		storage string
	}{
		{"cluster only", "/?cluster=prod-eks-1", "prod-eks-1", "", "", ""},
		{"q only", "/?q=2026", "", "2026", "", ""},
		{"all", "/?cluster=prod-eks-1&q=2026&window=7d&source=http&storage=local", "prod-eks-1", "2026", "http", "local"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", c.url, nil)
			f := ParseFilter(req)
			if f.Cluster != c.cluster {
				t.Fatalf("cluster: want %q, got %q", c.cluster, f.Cluster)
			}
			if f.Query != c.query {
				t.Fatalf("query: want %q, got %q", c.query, f.Query)
			}
			if f.Source != c.source {
				t.Fatalf("source: want %q, got %q", c.source, f.Source)
			}
			if f.Storage != c.storage {
				t.Fatalf("storage: want %q, got %q", c.storage, f.Storage)
			}
		})
	}
}

func TestParseFilterRejectsUnknownSourceAndStorage(t *testing.T) {
	req := httptest.NewRequest("GET", "/?source=pigeon&storage=lamp", nil)
	f := ParseFilter(req)
	if f.Source != "" || f.Storage != "" {
		t.Fatalf("unknown source/storage must be dropped: %+v", f)
	}
}

func TestQueryStringWith(t *testing.T) {
	req := httptest.NewRequest("GET", "/?cluster=foo&q=bar&window=7d", nil)
	got := QueryStringWith(req, "cluster", "baz")
	// Order in Encode is alphabetical; this is the canonical form.
	want := "cluster=baz&q=bar&window=7d"
	if got != want {
		t.Fatalf("QueryStringWith: want %q, got %q", want, got)
	}
}

func TestQueryStringWithEmptyRemoves(t *testing.T) {
	req := httptest.NewRequest("GET", "/?cluster=foo&q=bar", nil)
	got := QueryStringWith(req, "cluster", "")
	want := "q=bar"
	if got != want {
		t.Fatalf("QueryStringWith empty: want %q, got %q", want, got)
	}
}

func TestQueryStringWithout(t *testing.T) {
	req := httptest.NewRequest("GET", "/?cluster=foo&q=bar&window=7d", nil)
	got := QueryStringWithout(req, "q")
	want := "cluster=foo&window=7d"
	if got != want {
		t.Fatalf("QueryStringWithout: want %q, got %q", want, got)
	}
}

func TestFilterURLBuilderWithWithout(t *testing.T) {
	b := FilterURLBuilder{Base: "cluster=foo&q=bar"}
	if got := b.With("cluster", "baz"); got != "cluster=baz&q=bar" {
		t.Fatalf("With: %q", got)
	}
	if got := b.Without("q"); got != "cluster=foo" {
		t.Fatalf("Without: %q", got)
	}
}
