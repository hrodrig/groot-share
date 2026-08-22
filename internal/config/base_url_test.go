package config

import "testing"

func TestLoadFromEnvBaseURLValidation(t *testing.T) {
	dir := t.TempDir()

	cases := []struct {
		name    string
		env     string
		wantErr bool
	}{
		{"abs https", "https://share.example.com", false},
		{"abs http with port", "http://share.example.com:8080", false},
		{"abs https with path", "https://share.example.com/base", false},
		{"relative rejected", "/share", true},
		{"missing scheme rejected", "share.example.com", true},
		{"ftp rejected", "ftp://share.example.com", true},
		{"empty is allowed", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GFS_TOPOLOGY", "vps")
			t.Setenv("GFS_DATA_DIR", dir)
			t.Setenv("GFS_BASE_URL", tc.env)
			cfg, err := LoadFromEnv()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tc.env)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.BaseURL != tc.env {
				t.Fatalf("BaseURL %q want %q", cfg.BaseURL, tc.env)
			}
		})
	}
}
