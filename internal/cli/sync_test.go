// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// syncCaptureMu serializes tests that swap the process-global os.Stdout /
// os.Stderr to capture sync output. The sync command writes directly to these
// globals (not via cmd.OutOrStdout), so capture requires a process-wide swap
// that cannot run concurrently with other tests doing the same. Top-level Go
// test functions are sequential by default; the mutex keeps that contract
// explicit if any sibling ever adopts t.Parallel.
var syncCaptureMu sync.Mutex

// syncRunResult holds the captured output and exit classification of a
// driven `straddle sync` invocation.
type syncRunResult struct {
	stdout   []byte
	stderr   []byte
	err      error
	exitCode int
}

// runSyncCommand drives RootCmd() with the given sync args, capturing the
// process-global stdout/stderr that the sync command writes to. The DB is
// pointed at dbPath (a fresh temp file). Env is isolated so no real config
// or credentials are read. When baseURL is empty no request is made by the
// cases under test (the flat pool skips dependent names and the dependent
// runner no-ops on an empty parent table before any HTTP call), so the run
// is network-free; when baseURL points at an httptest server, flat top-level
// resources dial it for their list page.
func runSyncCommand(t *testing.T, dbPath, baseURL string, args ...string) syncRunResult {
	t.Helper()
	syncCaptureMu.Lock()
	defer syncCaptureMu.Unlock()

	configDir := t.TempDir()
	t.Setenv("HOME", configDir)
	t.Setenv("STRADDLE_CONFIG", filepath.Join(configDir, "config.toml"))
	t.Setenv("STRADDLE_PLATFORM_CONFIG", filepath.Join(configDir, "platform.toml"))
	t.Setenv("STRADDLE_API_KEY", "test_key")
	t.Setenv("STRADDLE_BASE_URL", baseURL)
	t.Setenv("STRADDLE_VERIFY", "")
	t.Setenv("STRADDLE_VERIFY_LIVE_HTTP", "")
	t.Setenv("NO_COLOR", "1")
	t.Setenv("TERM", "dumb")

	cmd := RootCmd()
	var stdout, stderr bytes.Buffer
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("capture stdout: %v", err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		_ = stdoutR.Close()
		_ = stdoutW.Close()
		t.Fatalf("capture stderr: %v", err)
	}
	stdoutDone := make(chan struct{})
	go func() { _, _ = io.Copy(&stdout, stdoutR); close(stdoutDone) }()
	stderrDone := make(chan struct{})
	go func() { _, _ = io.Copy(&stderr, stderrR); close(stderrDone) }()

	origStdout, origStderr := os.Stdout, os.Stderr
	origHumanFriendly := humanFriendly
	origNoColor := noColor
	origTerminal := isTerminalOverride
	os.Stdout = stdoutW
	os.Stderr = stderrW
	isTerminalOverride = nil
	noColor = false
	humanFriendly = false

	cmd.SetOut(stdoutW)
	cmd.SetErr(stderrW)
	cmd.SetIn(strings.NewReader(""))
	cmd.SetArgs(append([]string{"sync", "--db", dbPath}, args...))
	executeErr := cmd.Execute()

	_ = stdoutW.Close()
	_ = stderrW.Close()
	os.Stdout = origStdout
	os.Stderr = origStderr
	humanFriendly = origHumanFriendly
	noColor = origNoColor
	isTerminalOverride = origTerminal
	<-stdoutDone
	<-stderrDone
	_ = stdoutR.Close()
	_ = stderrR.Close()

	exitCode := 0
	if executeErr != nil {
		exitCode = ExitCode(executeErr)
	}
	return syncRunResult{
		stdout:   append([]byte(nil), stdout.Bytes()...),
		stderr:   append([]byte(nil), stderr.Bytes()...),
		err:      executeErr,
		exitCode: exitCode,
	}
}

// syncSummary is the JSON shape of the {"event":"sync_summary",...} line the
// sync command emits once per run.
type syncSummary struct {
	TotalRecords int `json:"total_records"`
	Resources    int `json:"resources"`
	Success      int `json:"success"`
	Warned       int `json:"warned"`
	Errored      int `json:"errored"`
	DurationMs   int `json:"duration_ms"`
}

// parseSyncSummary scans captured stdout for the single sync_summary event
// line and unmarshals it. Fails the test if it is absent or not unique.
func parseSyncSummary(t *testing.T, out []byte) syncSummary {
	t.Helper()
	var summary syncSummary
	found := 0
	for _, line := range bytes.Split(out, []byte("\n")) {
		if !bytes.Contains(line, []byte(`"event":"sync_summary"`)) {
			continue
		}
		found++
		if json.Unmarshal(line, &summary) != nil {
			t.Fatalf("malformed sync_summary line: %s", line)
		}
	}
	if found == 0 {
		t.Fatalf("no sync_summary event in stdout:\n%s", out)
	}
	if found > 1 {
		t.Fatalf("expected exactly one sync_summary event, got %d:\n%s", found, out)
	}
	return summary
}

// TestSyncResourcePathRejectsDependentNames pins the contract that motivates
// the enqueue-skip fix: every top-level resource resolves via syncResourcePath,
// and every dependent name does NOT. The flat worker pool resolves names
// through syncResourcePath, so a dependent name reaching the flat pool would
// error. If a dependent name were ever added to the flat path map, the
// enqueue-skip fix would silently bypass the dependent runner for it, so this
// guard makes that drift a test failure rather than a silent regression.
func TestSyncResourcePathRejectsDependentNames(t *testing.T) {
	t.Parallel()
	for _, r := range defaultSyncResources() {
		r := r
		t.Run("flat/"+r, func(t *testing.T) {
			t.Parallel()
			p, err := syncResourcePath(r)
			if err != nil {
				t.Fatalf("syncResourcePath(%q) unexpected error: %v", r, err)
			}
			if p == "" {
				t.Errorf("syncResourcePath(%q) returned empty path", r)
			}
		})
	}
	for _, dep := range dependentResourceDefs() {
		dep := dep
		t.Run("dependent/"+dep.Name, func(t *testing.T) {
			t.Parallel()
			if _, err := syncResourcePath(dep.Name); err == nil {
				t.Errorf("syncResourcePath(%q) = nil error, want error: dependent names must not resolve as flat resources (syncDependentResources owns them)", dep.Name)
			}
		})
	}
}

// TestSyncDependentByNameSkipsFlatPool verifies the reported bug's core
// reproduction: passing a dependent name (capability_requests) alone via
// --resources runs cleanly through the dependent runner instead of also
// being rejected by the flat pool's path map. This is the network-free
// lower bound of the documented-contract scenario (empty parent table),
// exercising the exact flag-parse -> flat-enqueue -> syncResourcePath ->
// exit-policy path. Before the fix each case produced errored=1 (flat
// "unknown sync resource") paired with success=1 (dependent no-op); the
// default mode emitted a misleading exit_policy_default_changed warning
// and --strict exited 1. After the fix errored=0 and both modes exit 0.
func TestSyncDependentByNameSkipsFlatPool(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"default JSON", []string{"--resources", "capability_requests"}},
		{"strict JSON", []string{"--strict", "--resources", "capability_requests"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			dbPath := filepath.Join(t.TempDir(), "data.db")
			res := runSyncCommand(t, dbPath, "", tc.args...)

			if res.err != nil {
				t.Fatalf("sync returned error %v (exit %d)\nstdout: %s\nstderr: %s",
					res.err, res.exitCode, res.stdout, res.stderr)
			}
			if res.exitCode != 0 {
				t.Errorf("exitCode = %d, want 0\nstdout: %s\nstderr: %s",
					res.exitCode, res.stdout, res.stderr)
			}

			combined := append([]byte(nil), res.stdout...)
			combined = append(combined, res.stderr...)
			if bytes.Contains(combined, []byte("unknown sync resource")) {
				t.Errorf("output must not contain the spurious flat-path error\nstdout: %s\nstderr: %s",
					res.stdout, res.stderr)
			}
			if bytes.Contains(res.stdout, []byte("exit_policy_default_changed")) {
				t.Errorf("output must not emit exit_policy_default_changed warning\nstdout: %s", res.stdout)
			}
			// The flat pool must not start a sync for the dependent name;
			// only the dependent runner processes it, and it emits no
			// sync_start. So no sync_start event should appear at all.
			if bytes.Contains(res.stdout, []byte(`"event":"sync_start"`)) {
				t.Errorf("stdout must not contain sync_start (dependent should be skipped by flat pool)\nstdout: %s", res.stdout)
			}

			summary := parseSyncSummary(t, res.stdout)
			if summary.Errored != 0 {
				t.Errorf("sync_summary errored = %d, want 0\nstdout: %s", summary.Errored, res.stdout)
			}
			if summary.Success != 1 {
				t.Errorf("sync_summary success = %d, want 1 (the dependent no-op)\nstdout: %s", summary.Success, res.stdout)
			}
			if summary.Resources != 1 {
				t.Errorf("sync_summary resources = %d, want 1\nstdout: %s", summary.Resources, res.stdout)
			}
		})
	}
}

// TestSyncFlatResourceStillEnqueued guards against the fix over-skipping: a
// top-level resource (accounts) named via --resources must still be enqueued
// to the flat pool and synced via syncResourcePath. A live httptest server
// returns an empty list page so the flat sync completes with zero records.
// The parent name also cascades to the capability_requests dependent, whose
// empty-parent no-op contributes a second success.
func TestSyncFlatResourceStillEnqueued(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[]"))
	}))
	defer server.Close()

	dbPath := filepath.Join(t.TempDir(), "data.db")
	res := runSyncCommand(t, dbPath, server.URL, "--resources", "accounts")

	if res.err != nil {
		t.Fatalf("sync returned error %v (exit %d)\nstdout: %s\nstderr: %s",
			res.err, res.exitCode, res.stdout, res.stderr)
	}
	if res.exitCode != 0 {
		t.Errorf("exitCode = %d, want 0\nstdout: %s\nstderr: %s", res.exitCode, res.stdout, res.stderr)
	}
	// accounts is a top-level resource: it must reach the flat pool and
	// emit sync_start, proving the enqueue-skip did not swallow flat names.
	if !bytes.Contains(res.stdout, []byte(`"event":"sync_start","resource":"accounts"`)) {
		t.Errorf("stdout should contain sync_start for accounts (flat pool must still run top-level resources)\nstdout: %s", res.stdout)
	}
	if bytes.Contains(res.stdout, []byte("unknown sync resource")) {
		t.Errorf("output must not contain unknown sync resource\nstdout: %s", res.stdout)
	}
	summary := parseSyncSummary(t, res.stdout)
	if summary.Errored != 0 {
		t.Errorf("sync_summary errored = %d, want 0\nstdout: %s", summary.Errored, res.stdout)
	}
	if summary.Success < 1 {
		t.Errorf("sync_summary success = %d, want >= 1\nstdout: %s", summary.Success, res.stdout)
	}
}
