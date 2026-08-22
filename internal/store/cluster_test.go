package store

import "testing"

func TestParseClusterSlug(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{"basic", "groot-prod-eks-1-20260821.tar.gz", "prod-eks-1", true},
		{"with timestamp suffix", "groot-prod-eks-1-202608211504.tar.gz", "prod-eks-1", true},
		{"with since marker", "groot-prod-eks-1-20260821-since-2h.tar.gz", "prod-eks-1", true},
		{"with since and full ts", "groot-stage-20260821150400-since-15m.tar.gz", "stage", true},
		{"multi-word cluster", "groot-my-cluster-9-20260821.tar.gz", "my-cluster-9", true},
		{"not a date", "groot-prod-eks-1.tar.gz", "", false},
		{"no extension", "groot-prod-eks-1-20260821", "prod-eks-1", true},
		{"empty", "", "", false},
		{"random", "hello-world.tar.gz", "", false},
		{"year only is not enough", "groot-cluster-2026.tar.gz", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := ParseClusterSlug(c.in)
			if got != c.want || ok != c.ok {
				t.Fatalf("ParseClusterSlug(%q) = (%q, %v), want (%q, %v)", c.in, got, ok, c.want, c.ok)
			}
		})
	}
}
