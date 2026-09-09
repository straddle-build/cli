// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestImportJSONDistinguishesDryRunWithoutSendingRequests(t *testing.T) {
	for _, mode := range []string{"--json", "--agent"} {
		for _, dryRun := range []bool{false, true} {
			name := mode + "/live"
			if dryRun {
				name = mode + "/dry-run"
			}
			t.Run(name, func(t *testing.T) {
				isolateAPIConfig(t)
				t.Setenv("HOME", t.TempDir())
				t.Setenv("STRADDLE_API_KEY", "test_key")
				var requests atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requests.Add(1)
					if r.Method != http.MethodPost || r.URL.Path != "/customers" {
						t.Errorf("unexpected import request: %s %s", r.Method, r.URL.Path)
					}
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"id":"created"}`))
				}))
				defer server.Close()
				t.Setenv("STRADDLE_BASE_URL", server.URL)
				input := filepath.Join(t.TempDir(), "customers.jsonl")
				if err := os.WriteFile(input, []byte("{\"name\":\"One\"}\n{\"name\":\"Two\"}\n# comment\ninvalid\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				args := []string{"import", "customers", "--input", input, mode}
				if dryRun {
					args = append(args, "--dry-run")
				}
				stdout, stderr, err, _ := capturedRun(t, args)
				if err != nil {
					t.Fatalf("import failed: %v; stderr=%s", err, stderr)
				}
				var result map[string]any
				if err := json.Unmarshal([]byte(stdout), &result); err != nil {
					t.Fatalf("invalid summary JSON: %v; stdout=%s", err, stdout)
				}
				if result["succeeded"] != float64(2) || result["failed"] != float64(1) || result["skipped"] != float64(1) {
					t.Fatalf("unexpected import counters: %v", result)
				}
				if dryRun {
					if result["dry_run"] != true || requests.Load() != 0 {
						t.Fatalf("dry run: summary=%v, requests=%d; want dry_run:true and no requests", result, requests.Load())
					}
				} else if _, marked := result["dry_run"]; marked || requests.Load() != 2 {
					t.Fatalf("live import: summary=%v, requests=%d; want existing summary and two requests", result, requests.Load())
				}
			})
		}
	}
}
