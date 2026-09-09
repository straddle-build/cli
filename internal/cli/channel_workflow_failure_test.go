// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/straddle-build/straddle-cli/internal/store"
)

func TestWorkflowArchiveReportsResourceFailures(t *testing.T) {
	for _, mode := range []string{"human", "--json", "--agent"} {
		for _, tc := range []struct {
			name       string
			status     int
			all        bool
			wantFailed bool
			wantSynced int
		}{
			{"success", http.StatusOK, false, false, 8},
			{"partial failure", http.StatusUnauthorized, false, true, 7},
			{"total failure", http.StatusUnauthorized, true, true, 0},
			{"access warning", http.StatusForbidden, false, false, 7},
		} {
			t.Run(mode+"/"+tc.name, func(t *testing.T) {
				isolateAPIConfig(t)
				t.Setenv("HOME", t.TempDir())
				t.Setenv("STRADDLE_API_KEY", "test_key")
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if tc.all || strings.HasSuffix(r.URL.Path, "/accounts") {
						if tc.status != http.StatusOK {
							http.Error(w, "resource denied", tc.status)
							return
						}
					}
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`[{"id":"kept"}]`))
				}))
				defer server.Close()
				t.Setenv("STRADDLE_BASE_URL", server.URL)
				dbPath := filepath.Join(t.TempDir(), "archive.db")
				args := []string{"workflow", "archive", "--db", dbPath}
				if mode != "human" {
					args = append(args, mode)
				}
				stdout, stderr, err, _ := capturedRun(t, args)
				if (err != nil) != tc.wantFailed {
					t.Errorf("archive error=%v, wantFailed=%v; stderr=%s", err, tc.wantFailed, stderr)
				}
				if tc.wantFailed && !strings.Contains(stderr, "accounts: error:") {
					t.Errorf("resource error missing from stderr: %s", stderr)
				}
				if mode == "human" {
					if !strings.HasPrefix(stdout, "Archived ") || strings.Count(stdout, "\n") != 1 {
						t.Fatalf("human stdout must remain one summary line: %q", stdout)
					}
				} else {
					var result struct {
						ResourcesSynced int `json:"resources_synced"`
						TotalItems      int `json:"total_items"`
					}
					if err := json.Unmarshal([]byte(stdout), &result); err != nil {
						t.Fatalf("stdout must remain one JSON document: %v; %s", err, stdout)
					}
					if result.ResourcesSynced != tc.wantSynced || result.TotalItems != tc.wantSynced {
						t.Errorf("summary=%+v, want %d synced resources and items", result, tc.wantSynced)
					}
				}
				db, err := store.Open(dbPath)
				if err != nil {
					t.Fatal(err)
				}
				defer db.Close()
				items, err := db.List("customers", 10)
				if err != nil {
					t.Fatal(err)
				}
				wantCustomers := 1
				if tc.all {
					wantCustomers = 0
				}
				if len(items) != wantCustomers {
					t.Errorf("preserved customers=%d, want %d", len(items), wantCustomers)
				}
			})
		}
	}
}
