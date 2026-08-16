package blob

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var datedHTTPKey = regexp.MustCompile(`\d{4}/\d{2}/\d{2}/[a-f0-9]{32}\.tar\.gz$`)

// NormalizePrefix ensures a trailing slash (default captures/).
func NormalizePrefix(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimLeft(p, "/")
	if p == "" {
		return "captures/"
	}
	if !strings.HasSuffix(p, "/") {
		p += "/"
	}
	return p
}

// UnderPrefix reports whether key is under NormalizePrefix(prefix).
// Rejects empty keys and any key containing "..".
func UnderPrefix(prefix, key string) bool {
	key = strings.TrimSpace(key)
	if key == "" || strings.Contains(key, "..") {
		return false
	}
	key = strings.TrimLeft(key, "/")
	return strings.HasPrefix(key, NormalizePrefix(prefix))
}

// HTTPKey is the gfs transit object key. groot upload.s3 uses a different shape.
func HTTPKey(prefix, id string, t time.Time) string {
	t = t.UTC()
	return fmt.Sprintf("%s%s/%s/%s/%s.tar.gz",
		NormalizePrefix(prefix),
		t.Format("2006"),
		t.Format("01"),
		t.Format("02"),
		id,
	)
}

// SourceForKey returns http for gfs dated keys, s3 for anything else under the prefix.
func SourceForKey(key string) string {
	if datedHTTPKey.MatchString(key) {
		return "http"
	}
	return "s3"
}
