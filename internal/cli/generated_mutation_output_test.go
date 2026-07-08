// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
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
	if env["action"] != "create" {
		t.Fatalf("action = %v, want create", env["action"])
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
