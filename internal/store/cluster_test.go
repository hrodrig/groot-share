package store

import "testing"

func TestParseClusterSlug(t *testing.T) {
	const doCluster = "e1359a66-dd34-49da-8740-519d490679b6.k8s.ondigitalocean.com-cluster"
	cases := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{"basic new format", "groot-capture-7kqv2xy-20260102-150405-prod.tar.gz", "prod", true},
		{"message before timestamp", "groot-capture-7kqv2xy-incident-42-20260102-150405-prod.tar.gz", "prod", true},
		{"with since before timestamp", "groot-capture-7kqv2xy-since-12h-20260102-150405-prod.tar.gz", "prod", true},
		{"since and message before timestamp", "groot-capture-7kqv2xy-since-12h-incident-42-20260102-150405-prod.tar.gz", "prod", true},
		{"DO-style cluster with UUID and dots", "groot-capture-jjye3aq-incident-42-20260825-191459-" + doCluster + ".tar.gz", doCluster, true},
		{"message is before timestamp, cluster last", "groot-capture-trigger-jjye3aq-develop-text-extra-opcional-que-viene-desde-groot-trigger-y-que-puede-tener-cualquier-cosa-incluido-develop-20260825-191459-prod.tar.gz", "prod", true},
		{"cluster may itself be named develop", "groot-capture-trigger-jjye3aq-20260825-191459-develop.tar.gz", "develop", true},
		{"multi-word cluster", "groot-capture-7kqv2xy-20260102-150405-my-cluster-9.tar.gz", "my-cluster-9", true},
		{"no extension", "groot-capture-7kqv2xy-20260102-150405-prod", "prod", true},
		{"not a timestamped capture", "groot-prod-eks-1.tar.gz", "", false},
		{"old positional format no longer supported", "groot-prod-eks-1-20260821.tar.gz", "", false},
		{"empty", "", "", false},
		{"random", "hello-world.tar.gz", "", false},
		{"year only is not enough", "groot-cluster-2026.tar.gz", "", false},
		{"timestamp but no cluster", "groot-capture-7kqv2xy-20260102-150405.tar.gz", "", false},
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
