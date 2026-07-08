// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/straddle-build/cli/internal/apisync"
)

func TestRunDriftAgentReportsNoShapeForIdenticalSpecs(t *testing.T) {
	t.Parallel()

	spec := writeCommandSpec(t, `{
		"openapi": "3.1.0",
		"paths": {
			"/v1/widgets": {
				"get": {
					"tags": ["Widgets"],
					"operationId": "ListWidgets",
					"summary": "List widgets"
				}
			}
		}
	}`)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := run([]string{"drift", "--base", spec, "--head", spec, "--agent"}, &stdout, &stderr); err != nil {
		t.Fatalf("run drift: %v\nstderr: %s", err, stderr.String())
	}

	var result apisync.DriftResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode drift JSON: %v\nstdout: %s", err, stdout.String())
	}
	if !result.NoDrift {
		t.Fatalf("NoDrift = false for identical specs: %#v", result)
	}
	if len(result.SupportedAdditions) != 0 || len(result.Changes) != 0 || len(result.Removals) != 0 || len(result.UnsupportedOperations) != 0 {
		t.Fatalf("identical specs emitted drift shape: %#v", result)
	}
}

func TestRunGenerateDryRunSelectsMissingSupportedOperationsDeterministically(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	cliDir := filepath.Join(repo, "internal", "cli")
	if err := os.MkdirAll(cliDir, 0o755); err != nil {
		t.Fatalf("mkdir internal/cli: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cliDir, "existing.go"), []byte(`package cli

var existingAnnotation = map[string]string{"straddle:endpoint": "widgets.list", "straddle:method": "GET", "straddle:path": "/v1/widgets"}
`), 0o644); err != nil {
		t.Fatalf("write existing annotation: %v", err)
	}
	spec := writeCommandSpec(t, `{
		"openapi": "3.1.0",
		"paths": {
			"/v1/zeta": {
				"post": {
					"tags": ["Zeta"],
					"operationId": "CreateZeta",
					"summary": "Create zeta",
					"requestBody": {"required": true, "content": {"application/json": {}}}
				}
			},
			"/v1/widgets": {
				"get": {
					"tags": ["Widgets"],
					"operationId": "ListWidgets",
					"summary": "List widgets"
				}
			},
			"/v1/alpha": {
				"post": {
					"tags": ["Alpha"],
					"operationId": "CreateAlpha",
					"summary": "Create alpha",
					"requestBody": {"required": true, "content": {"application/json": {}}}
				}
			},
			"/v1/upload": {
				"post": {
					"tags": ["Upload"],
					"operationId": "CreateUpload",
					"summary": "Upload a file",
					"requestBody": {"required": true, "content": {"multipart/form-data": {}}}
				}
			}
		}
	}`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := run([]string{"generate", "--spec", spec, "--repo", repo, "--dry-run", "--agent"}, &stdout, &stderr); err != nil {
		t.Fatalf("run generate: %v\nstderr: %s", err, stderr.String())
	}

	var result apisync.GenerateResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode generate JSON: %v\nstdout: %s", err, stdout.String())
	}
	if !result.DryRun {
		t.Fatalf("DryRun = false")
	}
	wantGenerated := []string{
		filepath.Join(cliDir, "alpha_create.go"),
		filepath.Join(cliDir, "zeta_create.go"),
	}
	if len(result.Generated) != len(wantGenerated) {
		t.Fatalf("Generated = %#v, want %#v", result.Generated, wantGenerated)
	}
	for i := range wantGenerated {
		if result.Generated[i] != wantGenerated[i] {
			t.Fatalf("Generated = %#v, want deterministic order %#v", result.Generated, wantGenerated)
		}
		if _, err := os.Stat(result.Generated[i]); !os.IsNotExist(err) {
			t.Fatalf("dry-run wrote %s, stat err %v", result.Generated[i], err)
		}
	}
	if len(result.UnsupportedOperations) != 1 {
		t.Fatalf("UnsupportedOperations = %#v, want one non-JSON request body", result.UnsupportedOperations)
	}
	unsupported := result.UnsupportedOperations[0]
	if unsupported.Operation.Key != "POST /v1/upload" {
		t.Fatalf("unsupported key = %q, want %q", unsupported.Operation.Key, "POST /v1/upload")
	}
	if len(unsupported.Reasons) != 1 || unsupported.Reasons[0] != "request body lacks application/json content" {
		t.Fatalf("unsupported reasons = %#v", unsupported.Reasons)
	}
}

func TestRunGenerateWritesGeneratedEndpointRegistration(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	cliDir := filepath.Join(repo, "internal", "cli")
	if err := os.MkdirAll(cliDir, 0o755); err != nil {
		t.Fatalf("mkdir internal/cli: %v", err)
	}
	spec := writeCommandSpec(t, `{
		"openapi": "3.1.0",
		"paths": {
			"/v1/widgets": {
				"post": {
					"tags": ["Widgets"],
					"operationId": "CreateWidgets",
					"summary": "Create widget",
					"requestBody": {"required": true, "content": {"application/json": {}}}
				}
			}
		}
	}`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := run([]string{"generate", "--spec", spec, "--repo", repo, "--agent"}, &stdout, &stderr); err != nil {
		t.Fatalf("run generate: %v\nstderr: %s", err, stderr.String())
	}

	generatedPath := filepath.Join(cliDir, "widgets_create.go")
	content, err := os.ReadFile(generatedPath)
	if err != nil {
		t.Fatalf("read generated file: %v", err)
	}
	got := string(content)
	if !strings.Contains(got, `registerGeneratedEndpoint("widgets.create", newWidgetsCreateCmd)`) {
		t.Fatalf("generated file missing registration call:\n%s", got)
	}
}

func writeCommandSpec(t *testing.T, data string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "spec.json")
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	return path
}
