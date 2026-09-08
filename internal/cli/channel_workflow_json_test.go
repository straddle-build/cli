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
	"testing"
)

// archiveMockEnv stands up an httptest server that returns a fixed
// 3-item array for every GET path and points the CLI at it, mirroring the
// golden/passthrough test pattern. The --data-source/--no-cache plumbing
// in the args forces the live fetch path against this mock.
func archiveMockEnv(t *testing.T) {
	t.Helper()
	isolateAPIConfig(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("NO_COLOR", "1")
	t.Setenv("TERM", "dumb")
	t.Setenv("STRADDLE_API_KEY", "test_key")
	body := []byte(`[{"id":"a"},{"id":"b"},{"id":"c"}]`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("unexpected %s request to %s", r.Method, r.URL.Path)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)
	t.Setenv("STRADDLE_BASE_URL", server.URL)
}

// capturedRun executes RootCmd with args while replacing os.Stdout and
// os.Stderr with pipes, so writers that bypass cmd.OutOrStdout() (notably
// syncResource's raw os.Stdout NDJSON emission) are still captured. The
// documented humanFriendly default (false, colors off, no --human-friendly)
// is forced to mirror the bare invocation the bug report leads with. It
// returns the captured streams, the RunE error, and the value of
// humanFriendly immediately after Execute so callers can assert no global
// side-effect leaks past the command.
func capturedRun(t *testing.T, args []string) (stdout, stderr string, runErr error, hfAfter bool) {
	t.Helper()
	cmd := RootCmd()
	var stdoutBuf, stderrBuf bytes.Buffer
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
		t.Fatalf("create stderr pipe: %v", err)
	}

	stdoutDone := make(chan struct{})
	go func() { _, _ = io.Copy(&stdoutBuf, stdoutReader); close(stdoutDone) }()
	stderrDone := make(chan struct{})
	go func() { _, _ = io.Copy(&stderrBuf, stderrReader); close(stderrDone) }()

	origStdout, origStderr := os.Stdout, os.Stderr
	origNoColor, origHF := noColor, humanFriendly
	origIsTerminal := isTerminalOverride
	origResource := currentResource
	terminal := false
	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter
	cmd.SetOut(stdoutWriter)
	cmd.SetErr(stderrWriter)
	cmd.SetIn(strings.NewReader(""))
	noColor = false
	humanFriendly = false
	isTerminalOverride = &terminal
	currentResource = ""
	cmd.SetArgs(args)

	runErr = cmd.Execute()
	hfAfter = humanFriendly

	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()
	os.Stdout, os.Stderr = origStdout, origStderr
	noColor, humanFriendly = origNoColor, origHF
	isTerminalOverride = origIsTerminal
	currentResource = origResource
	<-stdoutDone
	<-stderrDone
	_ = stdoutReader.Close()
	_ = stderrReader.Close()

	stdout = stdoutBuf.String()
	stderr = stderrBuf.String()
	return
}

func archiveArgs(t *testing.T, modeArgs ...string) []string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "data.db")
	args := append([]string{}, modeArgs...)
	args = append(args, "workflow", "archive", "--db", dbPath, "--no-cache", "--data-source", "live")
	return args
}

// TestWorkflowArchiveStdoutCleanBareHuman covers the documented primary
// invocation shown verbatim in the command's Example field: a bare
// `straddle workflow archive` with no flags. Before the fix, syncResource
// treated the default humanFriendly=false as machine mode and emitted
// NDJSON progress events straight to os.Stdout, polluting the human
// summary stream with 24 event fragments before the "Archived ..." line.
func TestWorkflowArchiveStdoutCleanBareHuman(t *testing.T) {
	archiveMockEnv(t)
	stdout, stderr, runErr, hfAfter := capturedRun(t, archiveArgs(t))
	if runErr != nil {
		t.Fatalf("workflow archive returned error: %v\nstderr: %s", runErr, stderr)
	}
	if strings.Contains(stdout, `{"event":"sync_`) {
		t.Errorf("stdout leaked NDJSON sync events; stdout:\n%s", stdout)
	}
	if !strings.HasPrefix(stdout, "Archived ") || !strings.Contains(stdout, "across") {
		t.Errorf("stdout missing human summary line; stdout:\n%s", stdout)
	}
	if !strings.Contains(stdout, "24 items") || !strings.Contains(stdout, "8 resources") {
		t.Errorf("stdout missing expected counts; stdout:\n%s", stdout)
	}
	if strings.Count(stdout, "\n") != 1 {
		t.Errorf("stdout should be a single summary line, got %d newlines; stdout:\n%s", strings.Count(stdout, "\n"), stdout)
	}
	// The fix toggles humanFriendly on for the sync loop and defers its
	// restoration; confirm the process-wide default did not leak past the
	// command (subsequent commands in the same process must stay unaffected).
	if hfAfter != false {
		t.Errorf("humanFriendly leaked after archive: got=%v want=false", hfAfter)
	}
}

// TestWorkflowArchiveStdoutCleanAgent covers --agent, the secondary
// manifestation. Before the fix the NDJSON event lines collided on stdout
// with archive's pretty-printed JSON summary object, yielding a stream
// that single-value JSON parsers (json.Unmarshal / jq / python json.load)
// reject with "extra data"/"invalid character after top-level value".
func TestWorkflowArchiveStdoutCleanAgent(t *testing.T) {
	archiveMockEnv(t)
	stdout, stderr, runErr, _ := capturedRun(t, archiveArgs(t, "--agent"))
	if runErr != nil {
		t.Fatalf("workflow archive --agent returned error: %v\nstderr: %s", runErr, stderr)
	}
	if strings.Contains(stdout, `{"event":"sync_`) {
		t.Errorf("stdout leaked NDJSON sync events under --agent; stdout:\n%s", stdout)
	}
	var summary map[string]any
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("stdout is not a single JSON object (a single-value parser must succeed): %v\nstdout:\n%s", err, stdout)
	}
	for _, key := range []string{"resources_synced", "total_items", "store_path", "timestamp"} {
		if _, ok := summary[key]; !ok {
			t.Errorf("summary object missing %q; stdout:\n%s", key, stdout)
		}
	}
	if got := summary["resources_synced"]; got != float64(8) {
		t.Errorf("resources_synced = %v, want 8", got)
	}
	if got := summary["total_items"]; got != float64(24) {
		t.Errorf("total_items = %v, want 24", got)
	}
}

// TestWorkflowArchiveStdoutCleanJSONFlag covers the --json path (without
// --agent). It shares the asJSON branch with --agent, but reaches it
// without the agent-only flag side effects, so it is exercised separately.
func TestWorkflowArchiveStdoutCleanJSONFlag(t *testing.T) {
	archiveMockEnv(t)
	stdout, stderr, runErr, _ := capturedRun(t, archiveArgs(t, "--json"))
	if runErr != nil {
		t.Fatalf("workflow archive --json returned error: %v\nstderr: %s", runErr, stderr)
	}
	if strings.Contains(stdout, `{"event":"sync_`) {
		t.Errorf("stdout leaked NDJSON sync events under --json; stdout:\n%s", stdout)
	}
	var summary map[string]any
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("stdout is not a single JSON object: %v\nstdout:\n%s", err, stdout)
	}
	if got := summary["resources_synced"]; got != float64(8) {
		t.Errorf("resources_synced = %v, want 8", got)
	}
	if got := summary["total_items"]; got != float64(24) {
		t.Errorf("total_items = %v, want 24", got)
	}
}

// TestSyncStillEmitsNDJSONContract guards that the one-file archive fix did
// not alter the sibling `straddle sync` command. sync's Long help documents
// NDJSON as its --json/--agent contract, and a final sync_summary aggregates
// the run; those events must still appear on stdout. This passes both
// before and after the archive fix — it is a regression guard, not a
// reproduction. --resources customers avoids the parent-keyed dependent
// walk so the assertion stays focused on sync's flat NDJSON contract.
func TestSyncStillEmitsNDJSONContract(t *testing.T) {
	archiveMockEnv(t)
	dbPath := filepath.Join(t.TempDir(), "sync.db")
	args := []string{
		"--agent", "sync",
		"--resources", "customers",
		"--concurrency", "1",
		"--db", dbPath,
		"--no-cache",
		"--data-source", "live",
	}
	stdout, stderr, runErr, _ := capturedRun(t, args)
	if runErr != nil {
		t.Fatalf("sync returned error: %v\nstderr: %s", runErr, stderr)
	}
	if !strings.Contains(stdout, `{"event":"sync_start","resource":"customers"}`) {
		t.Errorf("sync no longer emits sync_start on stdout; stdout:\n%s", stdout)
	}
	if !strings.Contains(stdout, `{"event":"sync_complete"`) {
		t.Errorf("sync no longer emits sync_complete on stdout; stdout:\n%s", stdout)
	}
	if !strings.Contains(stdout, `{"event":"sync_summary"`) {
		t.Errorf("sync no longer emits sync_summary on stdout; stdout:\n%s", stdout)
	}
}
