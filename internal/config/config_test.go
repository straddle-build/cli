// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package config

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestLoad_BaseURLTransportPolicy pins the credential transport boundary:
// non-loopback plaintext endpoints are rejected before the client can attach
// Authorization, while loopback HTTP remains available for hermetic tests and
// HTTPS production endpoints stay accepted.
func TestLoad_BaseURLTransportPolicy(t *testing.T) {
	cases := []struct {
		name        string
		baseURL     string
		wantErr     string
		wantBaseURL string
	}{
		{
			name:    "rejects non-loopback http",
			baseURL: "http://sandbox.straddle.com",
			wantErr: "must use https",
		},
		{
			name:        "accepts IPv4 loopback http",
			baseURL:     "http://127.0.0.1:8080",
			wantBaseURL: "http://127.0.0.1:8080",
		},
		{
			name:        "accepts localhost http",
			baseURL:     "http://localhost:3000",
			wantBaseURL: "http://localhost:3000",
		},
		{
			name:        "accepts https",
			baseURL:     "https://sandbox.straddle.com",
			wantBaseURL: "https://sandbox.straddle.com",
		},
		{
			name: "accepts templated default",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("STRADDLE_BASE_URL", tc.baseURL)
			t.Setenv("STRADDLE_API_KEY", "")
			t.Setenv("STRADDLE_CONFIG", "")
			t.Setenv("STRADDLE_ENVIRONMENT", "")

			cfg, err := Load(filepath.Join(t.TempDir(), "config.toml"))
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("Load() error = nil, want substring %q", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("Load() error = %q, want substring %q", err.Error(), tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if tc.wantBaseURL != "" && cfg.BaseURL != tc.wantBaseURL {
				t.Fatalf("BaseURL = %q, want %q", cfg.BaseURL, tc.wantBaseURL)
			}
		})
	}
}
