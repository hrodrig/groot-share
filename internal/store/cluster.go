package store

import (
	"regexp"
	"strings"
)

// clusterTSRegex matches the timestamp slot of a groot archive basename:
// `-<YYYYMMDD>-<HHMMSS>`. It is the positional anchor that separates the
// free-form leading segment (prefix, short id, optional since/message) from
// the trailing cluster segment. A `-since-<duration>` (when present) sits
// BEFORE the timestamp, so the timestamp is always cleanly delimited and the
// cluster is everything that follows it.
//
// Example: "groot-capture-7kqv2xy-rca-20260102-150405" matches the terminal
// `-20260102-150405` (group 1 = date, group 2 = time).
var clusterTSRegex = regexp.MustCompile(`-(\d{8})-(\d{6})`)

// ParseClusterSlug returns the cluster slug from a groot archive filename by
// POSITION, not by content: the cluster is everything AFTER the timestamp
// anchor (modulo the archive extension). This lets the cluster contain
// arbitrary characters — DO-style hosts, UUIDs, dashes, dots — without the
// parser guessing where the name ends.
//
// The parser is deliberately conservative: anything that does not contain a
// `-<YYYYMMDD>-<HHMMSS>` timestamp anchor returns "" rather than a guessed
// slug, so the dashboard count never inflates with junk values.
//
// Examples:
//
//	ParseClusterSlug("groot-capture-7kqv2xy-20260102-150405-prod.tar.gz")
//	  → "prod", true
//	ParseClusterSlug("groot-capture-jjye3aq-incident-42-20260825-191459-e1359a66-dd34-49da-8740-519d490679b6.k8s.ondigitalocean.com-cluster.tar.gz")
//	  → "e1359a66-dd34-49da-8740-519d490679b6.k8s.ondigitalocean.com-cluster", true
//	ParseClusterSlug("groot-capture-7kqv2xy-since-12h-20260102-150405-prod.tar.gz")
//	  → "prod", true
//	ParseClusterSlug("nope.tar.gz")
//	  → "", false
func ParseClusterSlug(key string) (string, bool) {
	base := key
	for _, ext := range []string{".tar.gz", ".tgz"} {
		base = strings.TrimSuffix(base, ext)
	}
	loc := clusterTSRegex.FindStringIndex(base)
	if loc == nil {
		return "", false
	}
	// Everything after the timestamp anchor is the cluster.
	slug := strings.TrimPrefix(base[loc[1]:], "-")
	if slug == "" {
		return "", false
	}
	return slug, true
}
