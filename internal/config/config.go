// Package config loads gfs settings from the environment.
package config

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

// Topology is the deploy-time storage layout. Not a per-upload flag.
type Topology string

const (
	// TopologyVPS stores archives on the VPS disk (home).
	TopologyVPS Topology = "vps"
	// TopologyVPSS3 uses VPS disk as transit and a bucket as home.
	TopologyVPSS3 Topology = "vps-s3"
)

// Config holds runtime settings for the gfs process.
type Config struct {
	ListenAddr string
	Topology   Topology
	DataDir    string

	LogFormat string // json | text
	LogLevel  string // debug|info|warn|error

	S3Bucket    string
	S3Region    string
	S3Endpoint  string
	S3Prefix    string
	S3PathStyle bool

	BootstrapAdmin    string
	BootstrapPassword string
	// BootstrapAdminName is the first admin display name. Empty → Administrator.
	BootstrapAdminName string
	CookieSecure       bool
	MaxUploadBytes     int64
	KeepLast           int
	MaxAgeDays         int
	RetentionEvery     time.Duration
	StagingGrace       time.Duration

	// LoginSimple hides product chrome on /login (no hero, no gfs title/favicon).
	LoginSimple bool
	// BrandSub replaces the app-bar tag ("archive door"). Empty → default. "-" hides.
	BrandSub string
	// Footer replaces the authenticated footer. Empty → default. "-" hides.
	Footer string

	// LoginRateLimit caps POST /login per client IP and per username (0 = disabled).
	LoginRateLimit LimitSpec
}

// LimitSpec is requests per window (0 = disabled).
type LimitSpec struct {
	Requests int
	Window   time.Duration
}

const (
	// DefaultBrandSub is the app-bar tag when GFS_BRAND_SUB is unset.
	DefaultBrandSub = "archive door"
	// DefaultBootstrapName is the first-admin display name when GFS_BOOTSTRAP_ADMIN_NAME is unset.
	DefaultBootstrapName = "Administrator"
	maxBrandSubRunes     = 32
	maxFooterRunes       = 120
	maxNameRunes         = 80
)

// LoadFromEnv reads configuration. Returns error if topology/data dir are
// invalid or if vps-s3 is missing required bucket/creds (fail closed).
func LoadFromEnv() (Config, error) {
	topo := Topology(strings.ToLower(strings.TrimSpace(os.Getenv("GFS_TOPOLOGY"))))
	cfg := Config{
		ListenAddr:         envOr("GFS_LISTEN", ":8080"),
		Topology:           topo,
		DataDir:            strings.TrimSpace(os.Getenv("GFS_DATA_DIR")),
		LogFormat:          strings.ToLower(envOr("GFS_LOG_FORMAT", "json")),
		LogLevel:           strings.ToLower(envOr("GFS_LOG_LEVEL", "info")),
		S3Bucket:           strings.TrimSpace(os.Getenv("GFS_S3_BUCKET")),
		S3Region:           envOr("GFS_S3_REGION", "us-east-1"),
		S3Endpoint:         strings.TrimSpace(os.Getenv("GFS_S3_ENDPOINT")),
		S3Prefix:           envOr("GFS_S3_PREFIX", "captures/"),
		BootstrapAdmin:     strings.TrimSpace(os.Getenv("GFS_BOOTSTRAP_ADMIN")),
		BootstrapPassword:  os.Getenv("GFS_BOOTSTRAP_PASSWORD"),
		BootstrapAdminName: ClipPlain(envOr("GFS_BOOTSTRAP_ADMIN_NAME", DefaultBootstrapName), maxNameRunes),
		CookieSecure:       parseBool(os.Getenv("GFS_COOKIE_SECURE"), false),
		MaxUploadBytes:     parseInt64(os.Getenv("GFS_MAX_UPLOAD_BYTES"), 32<<30),
		KeepLast:           parseInt(os.Getenv("GFS_KEEP_LAST"), 20),
		MaxAgeDays:         parseInt(os.Getenv("GFS_MAX_AGE_DAYS"), 90),
		RetentionEvery:     parseDuration(os.Getenv("GFS_RETENTION_EVERY"), time.Hour),
		StagingGrace:       parseDuration(os.Getenv("GFS_STAGING_GRACE"), 24*time.Hour),
		LoginSimple:        parseBool(os.Getenv("GFS_LOGIN_SIMPLE"), false),
		BrandSub:           strings.TrimSpace(os.Getenv("GFS_BRAND_SUB")),
		Footer:             strings.TrimSpace(os.Getenv("GFS_FOOTER")),
	}
	var err error
	cfg.LoginRateLimit, err = ParseLimitSpec(envOr("GFS_LOGIN_RATE_LIMIT", "20/1m"))
	if err != nil {
		return Config{}, fmt.Errorf("GFS_LOGIN_RATE_LIMIT: %w", err)
	}
	if cfg.Topology != TopologyVPS && cfg.Topology != TopologyVPSS3 {
		return Config{}, fmt.Errorf("GFS_TOPOLOGY is required (vps|vps-s3); %q is invalid (fail closed)", topo)
	}
	if cfg.DataDir == "" {
		return Config{}, fmt.Errorf("GFS_DATA_DIR is required (fail closed)")
	}
	if cfg.Topology == TopologyVPSS3 {
		if cfg.S3Bucket == "" {
			return Config{}, fmt.Errorf("GFS_S3_BUCKET is required for topology vps-s3 (fail closed)")
		}
		if strings.TrimSpace(os.Getenv("AWS_ACCESS_KEY_ID")) == "" || strings.TrimSpace(os.Getenv("AWS_SECRET_ACCESS_KEY")) == "" {
			return Config{}, fmt.Errorf("AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY are required for topology vps-s3 (fail closed)")
		}
	}
	pathStyleDefault := cfg.S3Endpoint != ""
	cfg.S3PathStyle = parseBool(os.Getenv("GFS_S3_PATH_STYLE"), pathStyleDefault)
	return cfg, nil
}

// ParseLimitSpec parses "N/1m", "N/1h", "N/30s", or "0" / empty / off (disabled).
func ParseLimitSpec(s string) (LimitSpec, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" || s == "0" || s == "off" || s == "disabled" {
		return LimitSpec{}, nil
	}
	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		return LimitSpec{}, fmt.Errorf("invalid limit %q (want N/duration e.g. 20/1m)", s)
	}
	n, err := strconv.Atoi(parts[0])
	if err != nil || n < 0 {
		return LimitSpec{}, fmt.Errorf("invalid request count in %q", s)
	}
	if n == 0 {
		return LimitSpec{}, nil
	}
	d, err := time.ParseDuration(parts[1])
	if err != nil || d <= 0 {
		return LimitSpec{}, fmt.Errorf("invalid duration in %q", s)
	}
	return LimitSpec{Requests: n, Window: d}, nil
}

// S3CredsPresent reports whether AWS keys are still set (readyz belt-and-suspenders).
func S3CredsPresent() bool {
	return strings.TrimSpace(os.Getenv("AWS_ACCESS_KEY_ID")) != "" &&
		strings.TrimSpace(os.Getenv("AWS_SECRET_ACCESS_KEY")) != ""
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func parseInt64(s string, def int64) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

func parseInt(s string, def int) int {
	n := parseInt64(s, int64(def))
	if n > math.MaxInt {
		return def
	}
	return int(n)
}

func parseDuration(s string, def time.Duration) time.Duration {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return def
	}
	return d
}

// DisplayBrandSub returns the app-bar tag. Empty env → default; "-" hides.
func DisplayBrandSub(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return DefaultBrandSub
	}
	return ClipPlain(raw, maxBrandSubRunes)
}

// DisplayUserName returns the bootstrap display name. Empty → Administrator.
func DisplayUserName(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return DefaultBootstrapName
	}
	return ClipPlain(raw, maxNameRunes)
}

// DisplayFooter returns custom footer text. Empty → default chrome (caller);
// "-" hides.
func DisplayFooter(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "-" {
		return ""
	}
	return ClipPlain(raw, maxFooterRunes)
}

// ClipPlain collapses whitespace, treats "-" as empty, and caps rune length.
func ClipPlain(s string, maxRunes int) string {
	s = strings.Join(strings.Fields(s), " ")
	if s == "-" {
		return ""
	}
	if maxRunes <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) > maxRunes {
		return string(r[:maxRunes])
	}
	return s
}

func parseBool(s string, def bool) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "":
		return def
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}
