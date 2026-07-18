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

func TestAPISyncWorkflowOnlyUpdatesLockfileForPureSupportedAdditions(t *testing.T) {
	t.Parallel()

	repo := testRepoRoot(t)
	data, err := os.ReadFile(filepath.Join(repo, ".github", "workflows", "api-sync.yml"))
	if err != nil {
		t.Fatalf("read api sync workflow: %v", err)
	}
	workflow := string(data)
	for _, want := range []string{
		"      - name: Generate supported endpoint additions\n        if: steps.drift.outputs.supported_additions != '0' && steps.drift.outputs.human_review_count == '0'",
		"      - name: Update spec lockfile\n        if: steps.drift.outputs.supported_additions != '0' && steps.drift.outputs.human_review_count == '0'",
		"      - name: Validate generated endpoint coverage\n        if: steps.drift.outputs.supported_additions != '0' && steps.drift.outputs.human_review_count == '0'",
		"go run ./cmd/gen-endpoint check",
		"${{ runner.temp }}/api-sync-coverage.json",
		`echo "- Supported additions: ${{ steps.drift.outputs.supported_additions }}"`,
	} {
		if !strings.Contains(workflow, want) {
			t.Fatalf("api sync workflow missing %q", want)
		}
	}
}

func TestAPISyncWorkflowQueuesGeneratedPullRequestForAutoMerge(t *testing.T) {
	t.Parallel()

	repo := testRepoRoot(t)
	data, err := os.ReadFile(filepath.Join(repo, ".github", "workflows", "api-sync.yml"))
	if err != nil {
		t.Fatalf("read api sync workflow: %v", err)
	}
	workflow := string(data)
	for _, want := range []string{
		"id: generated_pr",
		"      - name: Queue generated PR for auto-merge",
		"if: steps.generated_pr.outputs.pull-request-number != '' && env.DRY_RUN != 'true' && env.HAS_API_SYNC_BOT_TOKEN == 'true'",
		"GH_TOKEN: ${{ secrets.API_SYNC_BOT_TOKEN }}",
		"PR_NUMBER: ${{ steps.generated_pr.outputs.pull-request-number }}",
		"PR_HEAD_SHA: ${{ steps.generated_pr.outputs.pull-request-head-sha }}",
		"gh pr merge \"${PR_NUMBER}\" --auto --squash --delete-branch --match-head-commit \"${PR_HEAD_SHA}\"",
	} {
		if !strings.Contains(workflow, want) {
			t.Fatalf("api sync workflow missing %q", want)
		}
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

func TestClassifyDriftReportsRequestBodyRefAsUnsupported(t *testing.T) {
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
						"$ref": "#/components/requestBodies/CreateUpload"
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
	if len(result.SupportedAdditions) != 0 {
		t.Fatalf("SupportedAdditions = %#v, want request-body ref routed to unsupported", result.SupportedAdditions)
	}
	if len(result.UnsupportedOperations) != 1 {
		t.Fatalf("UnsupportedOperations = %#v, want one ref operation", result.UnsupportedOperations)
	}
	reasons := result.UnsupportedOperations[0].Reasons
	if !hasReasonContaining(reasons, "request body $ref is not supported: #/components/requestBodies/CreateUpload") {
		t.Fatalf("unsupported reasons = %#v, want request-body ref reason", reasons)
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

func TestUnsupportedReasonsRejectsGeneratedParameterNames(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		want string
	}{
		{
			name: "fields[]",
			want: `unsupported parameter name "fields[]"`,
		},
		{
			name: "3ds_version",
			want: `unsupported parameter name "3ds_version"`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			op := apisync.Operation{
				OperationID: "ListWidgets",
				Endpoint:    "widgets.list",
				Method:      "GET",
				Path:        "/v1/widgets",
				QueryParameters: []apisync.Parameter{
					{Name: tc.name, In: "query"},
				},
			}

			reasons := apisync.UnsupportedReasons(op)
			if !hasReasonContaining(reasons, tc.want) {
				t.Fatalf("UnsupportedReasons(%q) = %#v, want reason containing %q", tc.name, reasons, tc.want)
			}
		})
	}
}

func TestUnsupportedReasonsRejectsReservedGeneratedFlagNames(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"account", "json", "config", "help", "version"} {
		t.Run(name, func(t *testing.T) {
			op := apisync.Operation{
				OperationID: "ListWidgets",
				Endpoint:    "widgets.list",
				Method:      "GET",
				Path:        "/v1/widgets",
				QueryParameters: []apisync.Parameter{
					{Name: name, In: "query"},
				},
			}

			reasons := apisync.UnsupportedReasons(op)
			if !hasReasonContaining(reasons, `parameter flag name collision "`+name+`"`) {
				t.Fatalf("UnsupportedReasons(%q) = %#v, want reserved flag collision", name, reasons)
			}
		})
	}
}

func TestClassifyDriftRoutesGeneratedParameterCollisionsToUnsupported(t *testing.T) {
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
			"/v1/collisions": {
				"get": {
					"tags": ["Collisions"],
					"operationId": "ListCollisions",
					"summary": "List collisions",
					"parameters": [
						{"name": "request-id", "in": "query"},
						{"name": "request_id", "in": "query"}
					]
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
	if len(result.SupportedAdditions) != 0 {
		t.Fatalf("SupportedAdditions = %#v, want collision routed to unsupported", result.SupportedAdditions)
	}
	if len(result.UnsupportedOperations) != 1 {
		t.Fatalf("UnsupportedOperations = %#v, want one collision operation", result.UnsupportedOperations)
	}
	reasons := result.UnsupportedOperations[0].Reasons
	for _, want := range []string{"parameter flag name collision", "parameter variable name collision"} {
		if !hasReasonContaining(reasons, want) {
			t.Fatalf("unsupported reasons = %#v, want reason containing %q", reasons, want)
		}
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

func TestClassifyDriftReportsPathLevelParameterChange(t *testing.T) {
	t.Parallel()

	baseSpec := writeSpec(t, `{
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
	headSpec := writeSpec(t, `{
		"openapi": "3.1.0",
		"paths": {
			"/v1/widgets/{id}": {
				"parameters": [
					{"name": "id", "in": "path", "required": true, "description": "Updated widget id"}
				],
				"get": {
					"tags": ["Widgets"],
					"operationId": "GetWidget",
					"summary": "Get widget"
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
		t.Fatal("NoDrift = true, want path-level parameter change to be reported")
	}
	if len(result.Changes) != 1 {
		t.Fatalf("Changes = %d, want 1", len(result.Changes))
	}
	if result.Changes[0].Key != "GET /v1/widgets/{id}" {
		t.Fatalf("change key = %q, want GET /v1/widgets/{id}", result.Changes[0].Key)
	}
}

func TestGenerateEndpointFileUsesEmptyObjectForNoBodyMutation(t *testing.T) {
	t.Parallel()

	file, err := apisync.GenerateEndpointFile(apisync.Operation{
		OperationID: "ResubmitWidget",
		Endpoint:    "widgets.resubmit",
		Method:      "POST",
		Path:        "/v1/widgets/{id}/resubmit",
		PathParameters: []apisync.Parameter{
			{Name: "id", In: "path"},
		},
	}, t.TempDir())
	if err != nil {
		t.Fatalf("GenerateEndpointFile: %v", err)
	}
	got := file.Content
	for _, want := range []string{
		"body := map[string]any{}",
		"c.PostWithParamsAndHeaders(path, params, body, headers)",
		`return printGeneratedMutationOutput(cmd, flags, "POST", "widgets.resubmit", path, statusCode, data)`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated content missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "var body map[string]any") {
		t.Fatalf("generated content should not declare a typed nil body for no-body mutations:\n%s", got)
	}
}

func TestGenerateEndpointFileUsesDeleteClassifier(t *testing.T) {
	t.Parallel()

	file, err := apisync.GenerateEndpointFile(apisync.Operation{
		OperationID: "DeleteWidget",
		Endpoint:    "widgets.delete",
		Method:      "DELETE",
		Path:        "/v1/widgets/{id}",
		PathParameters: []apisync.Parameter{
			{Name: "id", In: "path"},
		},
	}, t.TempDir())
	if err != nil {
		t.Fatalf("GenerateEndpointFile: %v", err)
	}
	got := file.Content
	if !strings.Contains(got, "return classifyDeleteError(err, flags)") {
		t.Fatalf("generated content missing delete classifier:\n%s", got)
	}
	if !strings.Contains(got, `return printGeneratedMutationOutput(cmd, flags, "DELETE", "widgets.delete", path, statusCode, data)`) {
		t.Fatalf("generated DELETE content missing mutation output contract:\n%s", got)
	}
	if strings.Contains(got, "return classifyAPIError(err, flags)") {
		t.Fatalf("generated DELETE content should not use generic API classifier:\n%s", got)
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

func hasReasonContaining(reasons []string, want string) bool {
	for _, reason := range reasons {
		if strings.Contains(reason, want) {
			return true
		}
	}
	return false
}
