package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadFromEnvFailClosed(t *testing.T) {
	t.Setenv("GFS_LISTEN", "")
	t.Setenv("GFS_LOG_FORMAT", "")
	t.Setenv("GFS_LOG_LEVEL", "")
	t.Setenv("GFS_S3_BUCKET", "")
	t.Setenv("GFS_S3_REGION", "")
	t.Setenv("GFS_S3_ENDPOINT", "")
	t.Setenv("GFS_S3_PREFIX", "")
	t.Setenv("GFS_S3_PATH_STYLE", "")
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")

	cases := []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		{
			name:    "missing topology",
			env:     map[string]string{"GFS_TOPOLOGY": "", "GFS_DATA_DIR": t.TempDir()},
			wantErr: "GFS_TOPOLOGY",
		},
		{
			name:    "s3 alone",
			env:     map[string]string{"GFS_TOPOLOGY": "s3", "GFS_DATA_DIR": t.TempDir()},
			wantErr: "GFS_TOPOLOGY",
		},
		{
			name:    "unknown topology",
			env:     map[string]string{"GFS_TOPOLOGY": "laptop", "GFS_DATA_DIR": t.TempDir()},
			wantErr: "GFS_TOPOLOGY",
		},
		{
			name:    "missing data dir",
			env:     map[string]string{"GFS_TOPOLOGY": "vps", "GFS_DATA_DIR": ""},
			wantErr: "GFS_DATA_DIR",
		},
		{
			name: "vps-s3 missing bucket",
			env: map[string]string{
				"GFS_TOPOLOGY":          "vps-s3",
				"GFS_DATA_DIR":          t.TempDir(),
				"AWS_ACCESS_KEY_ID":     "ak",
				"AWS_SECRET_ACCESS_KEY": "sk",
			},
			wantErr: "GFS_S3_BUCKET",
		},
		{
			name: "vps-s3 missing creds",
			env: map[string]string{
				"GFS_TOPOLOGY":  "vps-s3",
				"GFS_DATA_DIR":  t.TempDir(),
				"GFS_S3_BUCKET": "captures",
			},
			wantErr: "AWS_ACCESS_KEY_ID",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GFS_TOPOLOGY", "")
			t.Setenv("GFS_DATA_DIR", "")
			t.Setenv("GFS_S3_BUCKET", "")
			t.Setenv("AWS_ACCESS_KEY_ID", "")
			t.Setenv("AWS_SECRET_ACCESS_KEY", "")
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			_, err := LoadFromEnv()
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err %v want substring %q", err, tc.wantErr)
			}
		})
	}
}

func TestLoadFromEnvVPS(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GFS_TOPOLOGY", "vps")
	t.Setenv("GFS_DATA_DIR", dir)
	t.Setenv("GFS_LISTEN", ":9090")
	t.Setenv("GFS_S3_BUCKET", "")
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Topology != TopologyVPS || cfg.ListenAddr != ":9090" || cfg.DataDir != dir {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
	if cfg.LogFormat != "json" || cfg.LogLevel != "info" {
		t.Fatalf("log defaults: %+v", cfg)
	}
}

func TestLoadFromEnvVPSS3(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GFS_TOPOLOGY", "vps-s3")
	t.Setenv("GFS_DATA_DIR", dir)
	t.Setenv("GFS_S3_BUCKET", "lab")
	t.Setenv("GFS_S3_ENDPOINT", "https://eu2.contabo.com")
	t.Setenv("GFS_S3_REGION", "")
	t.Setenv("GFS_S3_PREFIX", "")
	t.Setenv("GFS_S3_PATH_STYLE", "")
	t.Setenv("AWS_ACCESS_KEY_ID", "ak")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "sk")
	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Topology != TopologyVPSS3 || cfg.S3Bucket != "lab" {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
	if cfg.S3Region != "us-east-1" || cfg.S3Prefix != "captures/" {
		t.Fatalf("s3 defaults: %+v", cfg)
	}
	if !cfg.S3PathStyle {
		t.Fatal("path-style should default true when endpoint set")
	}
	if !S3CredsPresent() {
		t.Fatal("creds should be present")
	}
}

func TestParseBool(t *testing.T) {
	if parseBool("", true) != true || parseBool("false", true) != false {
		t.Fatal("parseBool")
	}
	if parseBool("yes", false) != true || parseBool("nope", false) != false {
		t.Fatal("parseBool fallback")
	}
}

func TestRetentionDefaults(t *testing.T) {
	t.Setenv("GFS_TOPOLOGY", "vps")
	t.Setenv("GFS_DATA_DIR", t.TempDir())
	t.Setenv("GFS_KEEP_LAST", "")
	t.Setenv("GFS_MAX_AGE_DAYS", "")
	t.Setenv("GFS_RETENTION_EVERY", "")
	t.Setenv("GFS_STAGING_GRACE", "")
	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.KeepLast != 20 || cfg.MaxAgeDays != 90 {
		t.Fatalf("defaults %+v", cfg)
	}
	t.Setenv("GFS_KEEP_LAST", "5")
	t.Setenv("GFS_MAX_AGE_DAYS", "7")
	cfg, err = LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.KeepLast != 5 || cfg.MaxAgeDays != 7 {
		t.Fatalf("override %+v", cfg)
	}
}

func TestLoadFromEnvInvalidKeepLast(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GFS_TOPOLOGY", "vps")
	t.Setenv("GFS_DATA_DIR", dir)
	t.Setenv("GFS_KEEP_LAST", "nope")
	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.KeepLast != 20 {
		t.Fatalf("default keep_last %+v", cfg)
	}
}

func TestLoadFromEnvMaxUpload(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GFS_TOPOLOGY", "vps")
	t.Setenv("GFS_DATA_DIR", dir)
	t.Setenv("GFS_MAX_UPLOAD_BYTES", "1048576")
	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxUploadBytes != 1048576 {
		t.Fatalf("max upload %+v", cfg)
	}
}

func TestLoadFromEnvDurations(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GFS_TOPOLOGY", "vps")
	t.Setenv("GFS_DATA_DIR", dir)
	t.Setenv("GFS_RETENTION_EVERY", "2h")
	t.Setenv("GFS_STAGING_GRACE", "bad")
	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RetentionEvery != 2*time.Hour {
		t.Fatalf("retention every %+v", cfg.RetentionEvery)
	}
	if cfg.StagingGrace != 24*time.Hour {
		t.Fatalf("staging grace default %+v", cfg.StagingGrace)
	}
}
