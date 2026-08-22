package store

import (
	"testing"
	"time"
)

func archiveWithKey(key string, createdAt time.Time) Archive {
	return Archive{ID: "id-" + key, Key: key, Size: 100, Source: "http", Storage: "local", CreatedAt: createdAt}
}

func TestApplyFilterEmptyIsNoOp(t *testing.T) {
	now := time.Date(2026, 8, 21, 19, 0, 0, 0, time.UTC)
	items := []Archive{
		archiveWithKey("groot-prod-eks-1-20260821.tar.gz", now),
	}
	got := applyFilter(items, Filter{})
	if len(got) != 1 {
		t.Fatalf("empty filter must not drop anything, got %d", len(got))
	}
}

func TestApplyFilterByCluster(t *testing.T) {
	now := time.Date(2026, 8, 21, 19, 0, 0, 0, time.UTC)
	items := []Archive{
		archiveWithKey("groot-prod-eks-1-20260821.tar.gz", now),
		archiveWithKey("groot-prod-eks-1-20260822.tar.gz", now),
		archiveWithKey("groot-stage-20260823.tar.gz", now),
		archiveWithKey("manual-upload.tar.gz", now),
	}
	got := applyFilter(items, Filter{Cluster: "prod-eks-1"})
	if len(got) != 2 {
		t.Fatalf("cluster filter: want 2, got %d", len(got))
	}
}

func TestApplyFilterByQuery(t *testing.T) {
	now := time.Date(2026, 8, 21, 19, 0, 0, 0, time.UTC)
	items := []Archive{
		archiveWithKey("groot-prod-eks-1-20260821.tar.gz", now),
		archiveWithKey("groot-prod-eks-1-20260822.tar.gz", now),
		archiveWithKey("groot-stage-20260823.tar.gz", now),
	}
	got := applyFilter(items, Filter{Query: "2026"})
	if len(got) != 3 {
		t.Fatalf("query 2026: want 3, got %d", len(got))
	}
	got = applyFilter(items, Filter{Query: "STAGE"})
	if len(got) != 1 {
		t.Fatalf("query STAGE (case-insensitive): want 1, got %d", len(got))
	}
}

func TestApplyFilterBySince(t *testing.T) {
	old := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	fresh := time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)
	items := []Archive{
		archiveWithKey("groot-prod-eks-1-20260801.tar.gz", old),
		archiveWithKey("groot-prod-eks-1-20260821.tar.gz", fresh),
	}
	cutoff := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	got := applyFilter(items, Filter{Since: cutoff})
	if len(got) != 1 {
		t.Fatalf("since filter: want 1, got %d", len(got))
	}
}

func TestApplyFilterBySource(t *testing.T) {
	now := time.Date(2026, 8, 21, 19, 0, 0, 0, time.UTC)
	items := []Archive{
		{Key: "groot-prod-eks-1-20260821.tar.gz", Source: "http", Storage: "local", CreatedAt: now},
		{Key: "groot-prod-eks-1-20260822.tar.gz", Source: "sftp", Storage: "local", CreatedAt: now},
		{Key: "groot-prod-eks-1-20260823.tar.gz", Source: "s3", Storage: "s3", CreatedAt: now},
	}
	got := applyFilter(items, Filter{Source: "sftp"})
	if len(got) != 1 {
		t.Fatalf("source sftp: want 1, got %d", len(got))
	}
}

func TestApplyFilterByStorage(t *testing.T) {
	now := time.Date(2026, 8, 21, 19, 0, 0, 0, time.UTC)
	items := []Archive{
		{Key: "a.tar.gz", Source: "http", Storage: "local", CreatedAt: now},
		{Key: "b.tar.gz", Source: "http", Storage: "s3", CreatedAt: now},
		{Key: "c.tar.gz", Source: "http", Storage: "transit", CreatedAt: now},
		{Key: "d.tar.gz", Source: "http", CreatedAt: now}, // empty → "local"
	}
	got := applyFilter(items, Filter{Storage: "transit"})
	if len(got) != 1 {
		t.Fatalf("storage transit: want 1, got %d", len(got))
	}
}

func TestApplyFilterCombined(t *testing.T) {
	now := time.Date(2026, 8, 21, 19, 0, 0, 0, time.UTC)
	old := now.Add(-48 * time.Hour)
	items := []Archive{
		archiveWithKey("groot-prod-eks-1-20260821.tar.gz", now), // cluster ok, fresh
		archiveWithKey("groot-prod-eks-1-20260819.tar.gz", old), // cluster ok, but old
		archiveWithKey("groot-stage-20260821.tar.gz", now),      // fresh, but wrong cluster
	}
	got := applyFilter(items, Filter{Cluster: "prod-eks-1", Since: now.Add(-24 * time.Hour)})
	if len(got) != 1 {
		t.Fatalf("combined cluster+since: want 1, got %d", len(got))
	}
}

func TestClusterCountsExcludesUnparsedKeys(t *testing.T) {
	now := time.Date(2026, 8, 21, 19, 0, 0, 0, time.UTC)
	items := []Archive{
		archiveWithKey("groot-prod-eks-1-20260821.tar.gz", now),
		archiveWithKey("groot-prod-eks-1-20260822.tar.gz", now),
		archiveWithKey("groot-prod-eks-1-20260823.tar.gz", now),
		archiveWithKey("groot-stage-20260821.tar.gz", now),
		archiveWithKey("manual-upload.tar.gz", now), // no timestamp
	}
	counts := ClusterCounts(items)
	if len(counts) != 2 {
		t.Fatalf("want 2 clusters, got %d: %+v", len(counts), counts)
	}
	// Sorted by count desc, then alpha asc: prod-eks-1 (3) before stage (1).
	if counts[0].Slug != "prod-eks-1" || counts[0].Count != 3 {
		t.Fatalf("counts[0]: %+v", counts[0])
	}
	if counts[1].Slug != "stage" || counts[1].Count != 1 {
		t.Fatalf("counts[1]: %+v", counts[1])
	}
}

func TestClusterCountsAlphaTiebreak(t *testing.T) {
	now := time.Date(2026, 8, 21, 19, 0, 0, 0, time.UTC)
	items := []Archive{
		archiveWithKey("groot-zeta-20260821.tar.gz", now),
		archiveWithKey("groot-alpha-20260821.tar.gz", now),
		archiveWithKey("groot-mike-20260821.tar.gz", now),
	}
	counts := ClusterCounts(items)
	// Same count → alpha asc: alpha, mike, zeta
	want := []string{"alpha", "mike", "zeta"}
	if len(counts) != 3 {
		t.Fatalf("want 3, got %d", len(counts))
	}
	for i, w := range want {
		if counts[i].Slug != w {
			t.Fatalf("[%d]: want %q, got %q", i, w, counts[i].Slug)
		}
	}
}
