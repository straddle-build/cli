// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/straddle-build/straddle-cli/internal/store"
)

func TestCashflowOutputHonorsCompactOnTerminal(t *testing.T) {
	for _, tc := range []struct {
		name          string
		terminal      bool
		humanFriendly bool
		args          []string
		wantJSON      bool
	}{
		{"terminal compact", true, false, []string{"--compact"}, true},
		{"terminal human-friendly compact", true, true, []string{"--compact", "--human-friendly"}, true},
		{"terminal default", true, false, nil, false},
		{"terminal JSON", true, false, []string{"--json"}, true},
		{"terminal agent", true, false, []string{"--agent"}, true},
		{"pipe default", false, false, nil, true},
		{"pipe compact", false, false, []string{"--compact"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolateAPIConfig(t)
			t.Setenv("HOME", t.TempDir())
			previousTerminal, previousHuman := isTerminalOverride, humanFriendly
			previousColor, previousResource := noColor, currentResource
			isTerminalOverride, humanFriendly = &tc.terminal, tc.humanFriendly
			t.Cleanup(func() {
				isTerminalOverride, humanFriendly = previousTerminal, previousHuman
				noColor, currentResource = previousColor, previousResource
			})
			dbPath := filepath.Join(t.TempDir(), "analytics.db")
			db, err := store.Open(dbPath)
			if err != nil {
				t.Fatal(err)
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			cmd := RootCmd()
			var stdout, stderr bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			cmd.SetArgs(append([]string{"cashflow", "--db", dbPath, "--days", "1"}, tc.args...))
			if err := cmd.Execute(); err != nil {
				t.Fatalf("cashflow failed: %v; stderr=%s", err, stderr.String())
			}
			if tc.wantJSON {
				var result cashflowResult
				if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
					t.Fatalf("expected machine JSON output: %v; stdout=%s", err, stdout.String())
				}
				if result.WindowDays != 1 || len(result.Buckets) != 1 || result.Net != 0 {
					t.Errorf("unexpected cashflow result: %+v", result)
				}
			} else if !strings.Contains(stdout.String(), "CHARGES") || !strings.Contains(stdout.String(), "Total in $0.00") {
				t.Errorf("expected human cashflow table, got %s", stdout.String())
			}
		})
	}
}
