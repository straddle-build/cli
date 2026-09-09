// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestPrintGeneratedMutationOutputSurfacesVerifyNoop(t *testing.T) {
	t.Parallel()

	cmd, stdout := generatedMutationOutputTestCommand()
	err := printGeneratedMutationOutput(cmd, &rootFlags{asJSON: true}, "POST", "widgets.create", "/v1/widgets", 200, json.RawMessage(`{"__straddle_verify_synthetic__":true,"id":"w_123"}`))
	if err != nil {
		t.Fatalf("printGeneratedMutationOutput: %v", err)
	}

	env := decodeGeneratedMutationEnvelope(t, stdout.String())
	if env["action"] != "post" {
		t.Fatalf("action = %v, want post", env["action"])
	}
	if env["resource"] != "widgets" {
		t.Fatalf("resource = %v, want widgets", env["resource"])
	}
	if env["path"] != "/v1/widgets" {
		t.Fatalf("path = %v, want /v1/widgets", env["path"])
	}
	if env["status"] != float64(200) {
		t.Fatalf("status = %v, want 200", env["status"])
	}
	if env["success"] != false {
		t.Fatalf("success = %v, want false", env["success"])
	}
	if env["verify_noop"] != true {
		t.Fatalf("verify_noop = %v, want true", env["verify_noop"])
	}
}

func TestPrintGeneratedMutationOutputReturnsPartialFailureExit(t *testing.T) {
	t.Parallel()

	cmd, stdout := generatedMutationOutputTestCommand()
	err := printGeneratedMutationOutput(cmd, &rootFlags{asJSON: true}, "PATCH", "widgets.update", "/v1/widgets/w_123", 200, json.RawMessage(`{"partialFailureError":{"message":"one operation failed","code":3},"results":[{"resourceName":"widgets/w_123"}]}`))
	if err == nil {
		t.Fatal("printGeneratedMutationOutput error = nil, want partial failure")
	}
	if ExitCode(err) != 6 {
		t.Fatalf("ExitCode(err) = %d, want 6; err=%v", ExitCode(err), err)
	}
	if !strings.Contains(err.Error(), "partial failure in widgets response: one operation failed") {
		t.Fatalf("err = %v, want widgets partial failure message", err)
	}

	env := decodeGeneratedMutationEnvelope(t, stdout.String())
	if env["success"] != false {
		t.Fatalf("success = %v, want false", env["success"])
	}
	if env["partial_failure"] == nil {
		t.Fatalf("partial_failure missing from envelope: %#v", env)
	}
}

func generatedMutationOutputTestCommand() (*cobra.Command, *bytes.Buffer) {
	cmd := &cobra.Command{Use: "test"}
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	return cmd, &stdout
}

func decodeGeneratedMutationEnvelope(t *testing.T, raw string) map[string]any {
	t.Helper()
	var env map[string]any
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		t.Fatalf("decode envelope %q: %v", raw, err)
	}
	return env
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = w
	defer func() { os.Stderr = orig }()
	out := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		out <- buf.String()
	}()
	fn()
	_ = w.Close()
	return <-out
}

// TestPrintGeneratedMutationOutputAllowPartialFailureEmitsWarningAndExits0
// drives printGeneratedMutationOutput with --allow-partial-failure and a
// partial-failure-shaped 2xx body across every output mode and asserts the
// documented contract: a warning is written to stderr AND the command exits 0.
// The warning is emitted unconditionally on detection, upstream of every
// output-mode branch, so it must appear for --quiet/--plain/--csv/--json and
// the human-table path alike. For envelope-producing modes the JSON success
// field must be false (a partial failure is never full success).
func TestPrintGeneratedMutationOutputAllowPartialFailureEmitsWarningAndExits0(t *testing.T) {
	// No t.Parallel(): captureStderr swaps the process-global os.Stderr, which
	// is unsafe to run concurrently with other stderr-capturing tests.
	body := json.RawMessage(`{"partialFailureError":{"message":"one operation failed","code":3},"data":[{"id":"w_1","name":"widget"}],"results":[{"resourceName":"widgets/w_1"}]}`)

	cases := []struct {
		name     string
		flags    *rootFlags
		terminal bool
		wantEnv  bool
	}{
		{name: "json", flags: &rootFlags{asJSON: true, allowPartialFailure: true}, wantEnv: true},
		{name: "pipe-default-envelope", flags: &rootFlags{allowPartialFailure: true}, wantEnv: true},
		{name: "quiet", flags: &rootFlags{quiet: true, allowPartialFailure: true}},
		{name: "plain", flags: &rootFlags{plain: true, allowPartialFailure: true}},
		{name: "csv", flags: &rootFlags{csv: true, allowPartialFailure: true}},
		{name: "human-table-terminal", flags: &rootFlags{allowPartialFailure: true}, terminal: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			terminal := tc.terminal
			isTerminalOverride = &terminal
			defer func() { isTerminalOverride = nil }()

			cmd, stdout := generatedMutationOutputTestCommand()
			var runErr error
			stderr := captureStderr(t, func() {
				runErr = printGeneratedMutationOutput(cmd, tc.flags, "POST", "widgets.create", "/v1/widgets", 200, body)
			})
			if runErr != nil {
				t.Fatalf("printGeneratedMutationOutput returned %v, want nil (exit 0 under --allow-partial-failure)", runErr)
			}
			if !strings.Contains(stderr, "warning: partial failure detected in widgets response: one operation failed") {
				t.Fatalf("stderr missing partial-failure warning; got:\n%s", stderr)
			}
			if !strings.Contains(stderr, "succeeded: 1 operation(s)") {
				t.Fatalf("stderr missing succeeded count; got:\n%s", stderr)
			}
			if tc.wantEnv {
				env := decodeGeneratedMutationEnvelope(t, stdout.String())
				if env["success"] != false {
					t.Fatalf("success = %v, want false (partial failure must not be reported as full success)", env["success"])
				}
				if env["partial_failure"] == nil {
					t.Fatalf("partial_failure missing from envelope: %#v", env)
				}
			}
		})
	}
}

// TestPrintGeneratedMutationOutputPartialFailureEmitsWarningWithoutFlag
// complements TestPrintGeneratedMutationOutputReturnsPartialFailureExit by
// asserting that the stderr warning is emitted UNCONDITIONALLY on detection,
// even when --allow-partial-failure is absent and the command exits non-zero
// (exit 6). This is the contract the surviving hand-authored bridge commands
// implement and that the generated path must honor.
func TestPrintGeneratedMutationOutputPartialFailureEmitsWarningWithoutFlag(t *testing.T) {
	// No t.Parallel(): captureStderr swaps the process-global os.Stderr.
	cmd, stdout := generatedMutationOutputTestCommand()
	body := json.RawMessage(`{"partialFailureError":{"message":"one operation failed","code":3},"results":[{"resourceName":"widgets/w_1"}]}`)

	var runErr error
	stderr := captureStderr(t, func() {
		runErr = printGeneratedMutationOutput(cmd, &rootFlags{asJSON: true}, "POST", "widgets.create", "/v1/widgets", 200, body)
	})
	if runErr == nil {
		t.Fatal("printGeneratedMutationOutput returned nil, want partial-failure error")
	}
	if got := ExitCode(runErr); got != 6 {
		t.Fatalf("ExitCode = %d, want 6; err=%v", got, runErr)
	}
	if !strings.Contains(stderr, "warning: partial failure detected in widgets response: one operation failed") {
		t.Fatalf("stderr missing partial-failure warning; got:\n%s", stderr)
	}
	if !strings.Contains(stderr, "succeeded: 1 operation(s)") {
		t.Fatalf("stderr missing succeeded count; got:\n%s", stderr)
	}
	env := decodeGeneratedMutationEnvelope(t, stdout.String())
	if env["success"] != false {
		t.Fatalf("success = %v, want false", env["success"])
	}
	if env["partial_failure"] == nil {
		t.Fatalf("partial_failure missing from envelope: %#v", env)
	}
}

// TestPrintGeneratedMutationOutputNoPartialFailureIsSilent guards against the
// warning being emitted when there is no partial failure: a clean 2xx body
// must not produce any stderr output and must report success=true.
func TestPrintGeneratedMutationOutputNoPartialFailureIsSilent(t *testing.T) {
	// No t.Parallel(): captureStderr swaps the process-global os.Stderr.
	cmd, stdout := generatedMutationOutputTestCommand()
	body := json.RawMessage(`{"id":"w_1","name":"widget"}`)

	var runErr error
	stderr := captureStderr(t, func() {
		runErr = printGeneratedMutationOutput(cmd, &rootFlags{asJSON: true, allowPartialFailure: true}, "POST", "widgets.create", "/v1/widgets", 200, body)
	})
	if runErr != nil {
		t.Fatalf("printGeneratedMutationOutput returned %v, want nil", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr output for a clean 2xx body; got:\n%s", stderr)
	}
	env := decodeGeneratedMutationEnvelope(t, stdout.String())
	if env["success"] != true {
		t.Fatalf("success = %v, want true for a clean 2xx body", env["success"])
	}
	if env["partial_failure"] != nil {
		t.Fatalf("partial_failure unexpectedly present: %#v", env["partial_failure"])
	}
}
