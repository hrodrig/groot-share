// Package config loads gfs settings from the environment.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
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
	CookieSecure      bool
	MaxUploadBytes    int64
}

// LoadFromEnv reads configuration. Returns error if topology/data dir are
// invalid or if vps-s3 is missing required bucket/creds (fail closed).
func LoadFromEnv() (Config, error) {
	topo := Topology(strings.ToLower(strings.TrimSpace(os.Getenv("GFS_TOPOLOGY"))))
	cfg := Config{
		ListenAddr:        envOr("GFS_LISTEN", ":8080"),
		Topology:          topo,
		DataDir:           strings.TrimSpace(os.Getenv("GFS_DATA_DIR")),
		LogFormat:         strings.ToLower(envOr("GFS_LOG_FORMAT", "json")),
		LogLevel:          strings.ToLower(envOr("GFS_LOG_LEVEL", "info")),
		S3Bucket:          strings.TrimSpace(os.Getenv("GFS_S3_BUCKET")),
		S3Region:          envOr("GFS_S3_REGION", "us-east-1"),
		S3Endpoint:        strings.TrimSpace(os.Getenv("GFS_S3_ENDPOINT")),
		S3Prefix:          envOr("GFS_S3_PREFIX", "captures/"),
		BootstrapAdmin:    strings.TrimSpace(os.Getenv("GFS_BOOTSTRAP_ADMIN")),
		BootstrapPassword: os.Getenv("GFS_BOOTSTRAP_PASSWORD"),
		CookieSecure:      parseBool(os.Getenv("GFS_COOKIE_SECURE"), false),
		MaxUploadBytes:    parseInt64(os.Getenv("GFS_MAX_UPLOAD_BYTES"), 32<<30),
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
