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
	t.Setenv("GFS_LOGIN_SIMPLE", "")
	t.Setenv("GFS_BRAND_SUB", "")
	t.Setenv("GFS_FOOTER", "")
	t.Setenv("GFS_BOOTSTRAP_ADMIN_NAME", "")
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
	if cfg.LoginSimple || cfg.BrandSub != "" || cfg.Footer != "" {
		t.Fatalf("brand defaults: %+v", cfg)
	}
	if cfg.BootstrapAdminName != DefaultBootstrapName {
		t.Fatalf("bootstrap name %q", cfg.BootstrapAdminName)
	}
}

func TestLoadFromEnvBranding(t *testing.T) {
	t.Setenv("GFS_TOPOLOGY", "vps")
	t.Setenv("GFS_DATA_DIR", t.TempDir())
	t.Setenv("GFS_LOGIN_SIMPLE", "true")
	t.Setenv("GFS_BRAND_SUB", "ACME CORP")
	t.Setenv("GFS_FOOTER", "Internal archive")
	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.LoginSimple || cfg.BrandSub != "ACME CORP" || cfg.Footer != "Internal archive" {
		t.Fatalf("branding: %+v", cfg)
	}
}

func TestLoadFromEnvBootstrapName(t *testing.T) {
	t.Setenv("GFS_TOPOLOGY", "vps")
	t.Setenv("GFS_DATA_DIR", t.TempDir())
	t.Setenv("GFS_BOOTSTRAP_ADMIN_NAME", "Ada Lovelace")
	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BootstrapAdminName != "Ada Lovelace" {
		t.Fatalf("name %q", cfg.BootstrapAdminName)
	}
}

func TestDisplayUserName(t *testing.T) {
	if DisplayUserName("") != DefaultBootstrapName {
		t.Fatal(DisplayUserName(""))
	}
	if DisplayUserName("  Ada  ") != "Ada" {
		t.Fatal(DisplayUserName("  Ada  "))
	}
}

func TestDisplayBrandSub(t *testing.T) {
	if DisplayBrandSub("") != DefaultBrandSub {
		t.Fatal(DisplayBrandSub(""))
	}
	if DisplayBrandSub("-") != "" {
		t.Fatal("hide")
	}
	if DisplayBrandSub("  ACME CORP  ") != "ACME CORP" {
		t.Fatal(DisplayBrandSub("  ACME CORP  "))
	}
}

func TestClipPlain(t *testing.T) {
	if ClipPlain("a\nb\tc", 32) != "a b c" {
		t.Fatal(ClipPlain("a\nb\tc", 32))
	}
	if got := ClipPlain("ABCDEFGHIJ", 4); got != "ABCD" {
		t.Fatal(got)
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

func TestParseIntRejectsOverflow(t *testing.T) {
	if parseInt("nope", 20) != 20 {
		t.Fatal("invalid")
	}
	if parseInt("0", 20) != 20 {
		t.Fatal("non-positive")
	}
	// Larger than int64 → ParseInt error → default (also covers 64→int bound).
	if parseInt("99999999999999999999", 20) != 20 {
		t.Fatal("overflow")
	}
	if parseInt("7", 20) != 7 {
		t.Fatal("ok")
	}
}

func TestLoadFromEnvKeepLastOverflow(t *testing.T) {
	t.Setenv("GFS_TOPOLOGY", "vps")
	t.Setenv("GFS_DATA_DIR", t.TempDir())
	t.Setenv("GFS_KEEP_LAST", "99999999999999999999")
	t.Setenv("GFS_MAX_AGE_DAYS", "99999999999999999999")
	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.KeepLast != 20 || cfg.MaxAgeDays != 90 {
		t.Fatalf("overflow should default %+v", cfg)
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
