package store

import (
	"regexp"
	"strings"
)

// clusterTSRegex matches the timestamp slot at the end of a groot archive
// basename (modulo extension). The full slot is `-<YYYYMMDD>[<sep>?<HHMMSS>]`
// where the separator is optional (some producers concatenate the time
// directly after the date).
//
// Example: "groot-prod-eks-1-20260821150405" matches at the trailing
// `-20260821150405`; group 1 is the date, group 2 is the time when present.
var clusterTSRegex = regexp.MustCompile(`-(\d{8})(?:[-]?(\d{4,14}))?$`)

// ParseClusterSlug returns the cluster slug from a groot archive filename,
// or "" when the name does not look like a timestamped capture. The parser
// is deliberately conservative: anything it does not recognise returns ""
// rather than a guessed slug, so the dashboard count never inflates with
// junk values.
//
// Examples:
//
//	ParseClusterSlug("groot-prod-eks-1-20260821.tar.gz")
//	  → "prod-eks-1", true
//	ParseClusterSlug("groot-prod-eks-1-202608211504.tar.gz")
//	  → "prod-eks-1", true
//	ParseClusterSlug("groot-prod-eks-1-20260821-since-2h.tar.gz")
//	  → "prod-eks-1", true
//	ParseClusterSlug("nope.tar.gz")
//	  → "", false
func ParseClusterSlug(key string) (string, bool) {
	base := key
	for _, ext := range []string{".tar.gz", ".tgz"} {
		base = strings.TrimSuffix(base, ext)
	}
	// Strip -since- marker first; it always trails the cluster in the
	// captured name. Without this, "groot-prod-eks-1-20260821-since-2h"
	// would not parse because the timestamp is no longer the terminal
	// segment.
	if i := strings.LastIndex(base, "-since-"); i >= 0 {
		base = base[:i]
	}
	loc := clusterTSRegex.FindStringIndex(base)
	if loc == nil {
		return "", false
	}
	prefix := base[:loc[0]] // everything before the timestamp marker
	parts := strings.Split(prefix, "-")
	if len(parts) < 2 {
		return "", false
	}
	// Drop the leading "groot" (or whatever the producer uses as the
	// archive prefix); the rest is the cluster slug, joined with "-".
	slug := strings.Join(parts[1:], "-")
	if slug == "" {
		return "", false
	}
	return slug, true
}
