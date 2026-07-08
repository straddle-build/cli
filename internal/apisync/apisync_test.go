// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package apisync_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/straddle-build/cli/internal/apisync"
)

func TestCurrentSpecOperationsAreCoveredByCheckedInAnnotations(t *testing.T) {
	t.Parallel()

	repo := testRepoRoot(t)
	ops, err := apisync.LoadSpec(filepath.Join(repo, "spec.json"))
	if err != nil {
		t.Fatalf("LoadSpec(spec.json): %v", err)
	}
	inventory, err := apisync.InventoryRepo(repo)
	if err != nil {
		t.Fatalf("InventoryRepo(%q): %v", repo, err)
	}

	result := apisync.CheckCoverage(ops, inventory)
	if !result.OK {
		t.Fatalf("current spec coverage failed: missing=%d extra=%d duplicate=%d invalid=%d", len(result.Missing), len(result.Extra), len(result.DuplicateAnnotations), len(result.InvalidAnnotations))
	}
}

func TestClassifyDriftReportsUnsupportedNonJSONRequestBodyAddition(t *testing.T) {
	t.Parallel()

	baseSpec := writeSpec(t, `{
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
	headSpec := writeSpec(t, `{
		"openapi": "3.1.0",
		"paths": {
			"/v1/uploads": {
				"post": {
					"tags": ["Uploads"],
					"operationId": "CreateUpload",
					"summary": "Upload a file",
					"requestBody": {
						"required": true,
						"content": {
							"multipart/form-data": {}
						}
					}
				}
			},
			"/v1/widgets": {
				"get": {
					"tags": ["Widgets"],
					"operationId": "ListWidgets",
					"summary": "List widgets"
				}
			}
		}
	}`)

	baseOps, err := apisync.LoadSpec(baseSpec)
	if err != nil {
		t.Fatalf("LoadSpec(base): %v", err)
	}
	headOps, err := apisync.LoadSpec(headSpec)
	if err != nil {
		t.Fatalf("LoadSpec(head): %v", err)
	}

	result := apisync.ClassifyDrift(baseOps, headOps)
	if result.NoDrift {
		t.Fatalf("NoDrift = true, want unsupported addition to be reported")
	}
	if len(result.SupportedAdditions) != 0 {
		t.Fatalf("SupportedAdditions = %d, want 0 for non-JSON request body", len(result.SupportedAdditions))
	}
	if len(result.UnsupportedOperations) != 1 {
		t.Fatalf("UnsupportedOperations = %d, want 1", len(result.UnsupportedOperations))
	}
	unsupported := result.UnsupportedOperations[0]
	if unsupported.Operation.Key != "POST /v1/uploads" {
		t.Fatalf("unsupported key = %q, want %q", unsupported.Operation.Key, "POST /v1/uploads")
	}
	if len(unsupported.Reasons) != 1 || unsupported.Reasons[0] != "request body lacks application/json content" {
		t.Fatalf("unsupported reasons = %#v", unsupported.Reasons)
	}
}

func TestUnsupportedReasonsRejectsJSONRequestBodyOnReadOperation(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		method string
		path   string
		reason string
	}{
		{
			method: "GET",
			path:   "/v1/search",
			reason: "request body is not supported for GET operations",
		},
		{
			method: "DELETE",
			path:   "/v1/widgets",
			reason: "request body is not supported for DELETE operations",
		},
	} {
		t.Run(tc.method, func(t *testing.T) {
			op := apisync.Operation{
				OperationID:           "ReadOperationWithBody",
				Method:                tc.method,
				Path:                  tc.path,
				RequestBodyRequired:   true,
				RequestBodyMediaTypes: []string{"application/json"},
			}

			reasons := apisync.UnsupportedReasons(op)
			if len(reasons) != 1 || reasons[0] != tc.reason {
				t.Fatalf("UnsupportedReasons(%s with JSON body) = %#v, want [%q]", tc.method, reasons, tc.reason)
			}
		})
	}
}

func TestParseSpecAppliesPathLevelParameters(t *testing.T) {
	t.Parallel()

	specPath := writeSpec(t, `{
		"openapi": "3.1.0",
		"paths": {
			"/v1/widgets/{id}": {
				"parameters": [
					{"name": "id", "in": "path", "required": true, "description": "Widget id"}
				],
				"get": {
					"tags": ["Widgets"],
					"operationId": "GetWidget",
					"summary": "Get widget"
				}
			}
		}
	}`)
	ops, err := apisync.LoadSpec(specPath)
	if err != nil {
		t.Fatalf("LoadSpec(path-level params): %v", err)
	}
	if len(ops) != 1 {
		t.Fatalf("operation count = %d, want 1", len(ops))
	}
	if len(ops[0].PathParameters) != 1 || ops[0].PathParameters[0].Name != "id" {
		t.Fatalf("path parameters = %#v, want id from path item", ops[0].PathParameters)
	}
}

func TestGenerateEndpointFileEmitsHeaderFlags(t *testing.T) {
	t.Parallel()

	file, err := apisync.GenerateEndpointFile(apisync.Operation{
		OperationID: "GetWidget",
		Endpoint:    "widgets.get",
		Method:      "GET",
		Path:        "/v1/widgets/{id}",
		PathParameters: []apisync.Parameter{
			{Name: "id", In: "path"},
		},
		HeaderParameters: []apisync.Parameter{
			{Name: "Straddle-Account-Id", In: "header"},
			{Name: "Request-Id", In: "header", Description: "Trace one request."},
		},
	}, t.TempDir())
	if err != nil {
		t.Fatalf("GenerateEndpointFile: %v", err)
	}
	got := file.Content
	for _, want := range []string{
		"var flagRequestIdHeader string",
		`headers["Request-Id"] = flagRequestIdHeader`,
		`c.GetWithHeaders(path, params, headers)`,
		`cmd.Flags().StringVar(&flagRequestIdHeader, "request-id", "", "Trace one request.")`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated content missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Straddle-Account-Id") {
		t.Fatalf("generated content should not expose Straddle-Account-Id as a header flag:\n%s", got)
	}
}

func testRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func writeSpec(t *testing.T, data string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "spec.json")
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	return path
}
