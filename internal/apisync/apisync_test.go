// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package apisync_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/straddle-build/straddle-cli/internal/apisync"
	"sigs.k8s.io/yaml"
)

func TestCurrentSpecOperationsAreCoveredByCheckedInAnnotations(t *testing.T) {
	t.Parallel()

	repo := testRepoRoot(t)
	result, err := apisync.CheckSpecAgainstRepo(filepath.Join(repo, "spec.yaml"), repo)
	if err != nil {
		t.Fatalf("CheckSpecAgainstRepo: %v", err)
	}
	allowReviewDrift := os.Getenv("STRADDLE_API_SYNC_REVIEW") == "true"
	if !coverageAccepted(result, allowReviewDrift) {
		t.Fatalf("current spec coverage failed: missing=%d extra=%d duplicate=%d invalid=%d operation_id_mismatch=%d", len(result.Missing), len(result.Extra), len(result.DuplicateAnnotations), len(result.InvalidAnnotations), len(result.OperationIDMismatches))
	}
	if !unsupportedInventoryAccepted(len(result.UnsupportedOperations), allowReviewDrift) {
		t.Fatalf("unsupported operations = %d, want two unsupported contract operations", len(result.UnsupportedOperations))
	}
}

func TestAPISyncReviewModeAllowsOnlyRemovedOrRenamedOperations(t *testing.T) {
	t.Parallel()

	reviewable := apisync.CheckResult{
		Extra:                 []apisync.Annotation{{File: "removed.go"}},
		OperationIDMismatches: []apisync.OperationIDMismatch{{Key: "GET /v1/widgets"}},
	}
	if coverageAccepted(reviewable, false) {
		t.Fatal("strict coverage accepted review-only drift")
	}
	if !coverageAccepted(reviewable, true) {
		t.Fatal("API sync review mode rejected removed or renamed operations")
	}
	reviewable.Missing = []apisync.Operation{{Key: "POST /v1/widgets"}}
	if coverageAccepted(reviewable, true) {
		t.Fatal("API sync review mode accepted a missing supported operation")
	}
	if unsupportedInventoryAccepted(3, false) {
		t.Fatal("strict coverage accepted an unexpected unsupported operation")
	}
	if !unsupportedInventoryAccepted(3, true) {
		t.Fatal("API sync review mode rejected an unsupported operation for human review")
	}
}

func coverageAccepted(result apisync.CheckResult, allowReviewDrift bool) bool {
	if result.HasBlockingIssues() {
		return false
	}
	return allowReviewDrift || result.OK
}

func unsupportedInventoryAccepted(count int, allowReviewDrift bool) bool {
	return allowReviewDrift || count == 2
}

func TestParseSpecLoadsYAMLAndResolvesSharedParameters(t *testing.T) {
	t.Parallel()

	operations, err := apisync.ParseSpec([]byte(`
openapi: 3.1.0
info: {version: 1.2.3}
paths:
  /v1/widgets/{id}:
    get:
      tags: [widgets]
      operationId: getWidget
      parameters:
        - {name: id, in: path, required: true, schema: {type: string}}
        - {$ref: '#/components/parameters/RequestId'}
components:
  parameters:
    RequestId:
      name: Request-Id
      in: header
      schema: {type: string}
`))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}
	if len(operations) != 1 {
		t.Fatalf("operations = %d, want 1", len(operations))
	}
	operation := operations[0]
	if operation.OperationID != "getWidget" || len(operation.PathParameters) != 1 || len(operation.HeaderParameters) != 1 {
		t.Fatalf("operation = %#v", operation)
	}
	if operation.HeaderParameters[0].Name != "Request-Id" {
		t.Fatalf("header parameter = %#v, want resolved Request-Id", operation.HeaderParameters[0])
	}
}

func TestCheckCoverageUsesOperationIDsAndIgnoresInternalCommands(t *testing.T) {
	t.Parallel()

	operations := []apisync.Operation{{Key: "GET /v1/widgets", OperationID: "getWidget", Method: "GET", Path: "/v1/widgets"}}
	inventory := apisync.Inventory{Annotations: []apisync.Annotation{
		{OperationID: "wrongOperation", Method: "GET", Path: "/v1/widgets", File: "widgets.go"},
		{Method: "POST", Path: "/v1/internal", File: "internal.go", Internal: true},
	}}
	result := apisync.CheckCoverage(operations, inventory)
	if result.OK || len(result.OperationIDMismatches) != 1 {
		t.Fatalf("result = %#v, want one operationId mismatch", result)
	}
	if len(result.Extra) != 0 || result.AnnotatedEndpoints != 1 {
		t.Fatalf("internal command affected contract coverage: %#v", result)
	}
}

func TestCheckSpecAgainstRepoTreatsAnnotatedUnsupportedOperationAsExtra(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	cliDir := filepath.Join(repo, "internal", "cli")
	if err := os.MkdirAll(cliDir, 0o755); err != nil {
		t.Fatalf("create cli directory: %v", err)
	}
	annotation := `package cli

var commandAnnotation = map[string]string{
	"straddle:endpoint": "widgets.create",
	"straddle:operation-id": "createWidget",
	"straddle:method": "POST",
	"straddle:path": "/v1/widgets",
}
`
	if err := os.WriteFile(filepath.Join(cliDir, "widgets.go"), []byte(annotation), 0o600); err != nil {
		t.Fatalf("write annotation: %v", err)
	}
	specPath := writeSpec(t, `{
		"openapi": "3.1.0",
		"paths": {
			"/v1/widgets": {
				"post": {
					"operationId": "createWidget",
					"tags": ["widgets"],
					"requestBody": {
						"content": {
							"application/json": {
								"schema": {"$ref": "#/components/schemas/CreateWidget"}
							}
						}
					}
				}
			}
		},
		"components": {
			"schemas": {
				"CreateWidget": {
					"type": "object",
					"properties": {
						"mystery": {}
					}
				}
			}
		}
	}`)

	result, err := apisync.CheckSpecAgainstRepo(specPath, repo)
	if err != nil {
		t.Fatalf("CheckSpecAgainstRepo: %v", err)
	}
	if result.OK {
		t.Fatalf("OK = true, want false: %#v", result)
	}
	if len(result.UnsupportedOperations) != 1 {
		t.Fatalf("UnsupportedOperations = %#v, want one", result.UnsupportedOperations)
	}
	unsupported := result.UnsupportedOperations[0]
	if unsupported.Operation.Key != "POST /v1/widgets" || !hasReasonContaining(unsupported.Reasons, "/mystery") {
		t.Fatalf("unsupported operation = %#v, want referenced schema reason", unsupported)
	}
	if len(result.Extra) != 1 || result.Extra[0].OperationID != "createWidget" {
		t.Fatalf("Extra = %#v, want stale createWidget annotation", result.Extra)
	}
	if len(result.Missing) != 0 {
		t.Fatalf("Missing = %#v, want unsupported operation excluded from coverage", result.Missing)
	}
}

func TestCheckSpecAgainstRepoReportsStaleGeneratedSurface(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	specPath := writeSpec(t, `{
		"openapi": "3.1.0",
		"paths": {
			"/v1/widgets": {
				"get": {
					"operationId": "listWidgets",
					"tags": ["widgets"],
					"parameters": [
						{"name": "status", "in": "query", "schema": {"type": "string", "default": "active"}},
						{"name": "Straddle-Account-Id", "in": "header", "schema": {"type": "string"}}
					]
				}
			}
		}
	}`)
	generated, err := apisync.GenerateAll(specPath, repo, false)
	if err != nil {
		t.Fatalf("GenerateAll: %v", err)
	}
	if len(generated.Generated) != 1 {
		t.Fatalf("Generated = %#v, want one file", generated.Generated)
	}
	path := generated.Generated[0]
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read generated file: %v", err)
	}
	stale := strings.ReplaceAll(string(content), `Default: "active"`, `Default: "inactive"`)
	stale = strings.ReplaceAll(stale, "AcceptsAccountHeader: true", "AcceptsAccountHeader: false")
	if stale == string(content) || !strings.Contains(stale, `Default: "inactive"`) || !strings.Contains(stale, "AcceptsAccountHeader: false") {
		t.Fatalf("generated fixture did not contain expected surface values:\n%s", content)
	}
	if err := os.WriteFile(path, []byte(stale), 0o644); err != nil {
		t.Fatalf("write stale generated file: %v", err)
	}

	result, err := apisync.CheckSpecAgainstRepo(specPath, repo)
	if err != nil {
		t.Fatalf("CheckSpecAgainstRepo: %v", err)
	}
	if result.OK || !result.HasBlockingIssues() {
		t.Fatalf("result = %#v, want blocking stale generated file", result)
	}
	if want := []string{path}; !reflect.DeepEqual(result.StaleGenerated, want) {
		t.Fatalf("StaleGenerated = %#v, want %#v", result.StaleGenerated, want)
	}
}

func TestCheckSpecAgainstRepoDedupesStaleGeneratedOnEndpointCollision(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	cliDir := filepath.Join(repo, "internal", "cli")
	if err := os.MkdirAll(cliDir, 0o755); err != nil {
		t.Fatalf("create cli directory: %v", err)
	}

	// v1: CreateThing is supported (JSON body) and owns internal/cli/things_create.go.
	v1Spec := writeSpec(t, `{
		"openapi": "3.1.0",
		"paths": {
			"/v1/things": {
				"post": {
					"tags": ["things"],
					"operationId": "CreateThing",
					"summary": "Create a thing",
					"requestBody": {
						"required": true,
						"content": {
							"application/json": {
								"schema": {"$ref": "#/components/schemas/CreateThing"}
							}
						}
					}
				}
			}
		},
		"components": {
			"schemas": {
				"CreateThing": {
					"type": "object",
					"properties": {"name": {"type": "string"}},
					"required": ["name"]
				}
			}
		}
	}`)
	if _, err := apisync.GenerateAll(v1Spec, repo, false); err != nil {
		t.Fatalf("GenerateAll v1: %v", err)
	}
	collisionPath := filepath.Join(cliDir, "things_create.go")
	if _, err := os.Stat(collisionPath); err != nil {
		t.Fatalf("v1 did not materialize %s: %v", collisionPath, err)
	}

	// v2: CreateThing drops application/json (now unsupported) while a new
	// supported CreateThings operation derives to the same endpoint/filename.
	v2Spec := writeSpec(t, `{
		"openapi": "3.1.0",
		"paths": {
			"/v1/things": {
				"post": {
					"tags": ["things"],
					"operationId": "CreateThing",
					"summary": "Create a thing",
					"requestBody": {
						"required": true,
						"content": {
							"multipart/form-data": {"schema": {"type": "object"}}
						}
					}
				}
			},
			"/v1/things/bulk": {
				"post": {
					"tags": ["things"],
					"operationId": "CreateThings",
					"summary": "Create things in bulk",
					"requestBody": {
						"required": true,
						"content": {
							"application/json": {
								"schema": {"$ref": "#/components/schemas/CreateThings"}
							}
						}
					}
				}
			}
		},
		"components": {
			"schemas": {
				"CreateThings": {
					"type": "object",
					"properties": {"name": {"type": "string"}},
					"required": ["name"]
				}
			}
		}
	}`)

	// Precondition: both operations collapse to the same endpoint (things.create),
	// so GenerateAll would otherwise record things_create.go in both Deleted and Generated.
	v2Ops, err := apisync.LoadSpec(v2Spec)
	if err != nil {
		t.Fatalf("LoadSpec v2: %v", err)
	}
	endpoints := map[string]string{}
	for _, op := range v2Ops {
		endpoints[op.OperationID] = op.Endpoint
	}
	if endpoints["CreateThing"] != "things.create" || endpoints["CreateThings"] != "things.create" {
		t.Fatalf("expected both operations to derive to things.create, got %#v", endpoints)
	}

	// Hand-authored annotation for the supported CreateThings operation keeps the
	// collision isolated from coverage gaps (Missing must stay empty).
	annotation := `package cli

var thingsBulkAnnotation = map[string]string{
	"straddle:endpoint": "things.create",
	"straddle:operation-id": "CreateThings",
	"straddle:method": "POST",
	"straddle:path": "/v1/things/bulk",
}
`
	if err := os.WriteFile(filepath.Join(cliDir, "things_bulk.go"), []byte(annotation), 0o600); err != nil {
		t.Fatalf("write annotation: %v", err)
	}

	result, err := apisync.CheckSpecAgainstRepo(v2Spec, repo)
	if err != nil {
		t.Fatalf("CheckSpecAgainstRepo: %v", err)
	}
	if len(result.Missing) != 0 {
		t.Fatalf("Missing = %#v, want empty so the collision is isolated to StaleGenerated", result.Missing)
	}
	if len(result.StaleGenerated) != 1 {
		t.Fatalf("StaleGenerated = %#v, want exactly one %s (deduplicated)", result.StaleGenerated, collisionPath)
	}
	if result.StaleGenerated[0] != collisionPath {
		t.Fatalf("StaleGenerated[0] = %q, want %q", result.StaleGenerated[0], collisionPath)
	}
	if !result.HasBlockingIssues() {
		t.Fatalf("HasBlockingIssues = false, want stale generated file to block")
	}
}

func TestAPISyncWorkflowAlwaysProposesNewPublishedContracts(t *testing.T) {
	t.Parallel()

	workflow := readWorkflow(t, filepath.Join(testRepoRoot(t), ".github", "workflows", "api-sync.yml"))
	if len(workflow.On.Schedule) != 1 {
		t.Fatalf("schedule triggers = %d, want 1", len(workflow.On.Schedule))
	}
	if got := workflow.On.RepositoryDispatch.Types; len(got) != 1 || got[0] != "straddle-contract-published" {
		t.Fatalf("repository dispatch types = %#v", got)
	}
	versionInput := workflow.On.WorkflowDispatch.Inputs["contract_version"]
	if !versionInput.Required || versionInput.Type != "string" {
		t.Fatalf("contract_version input = %#v, want required string", versionInput)
	}

	steps := stepsByName(workflow.Jobs["sync"])
	if steps["Update pinned contract"].If != "steps.contract.outputs.status == 'new'" {
		t.Fatalf("update condition = %q", steps["Update pinned contract"].If)
	}
	generateCondition := steps["Regenerate endpoint commands"].If
	if generateCondition != "steps.contract.outputs.status == 'new'" {
		t.Fatalf("generation condition = %q", generateCondition)
	}

	openPR := steps["Open contract synchronization pull request"]
	if openPR.Uses != "peter-evans/create-pull-request@v8" {
		t.Fatalf("PR action = %q", openPR.Uses)
	}
	if openPR.If != "steps.contract.outputs.status == 'new' && env.DRY_RUN != 'true' && env.HAS_API_SYNC_BOT_TOKEN == 'true'" {
		t.Fatalf("PR condition = %q", openPR.If)
	}
	branch, ok := openPR.With["branch"].(string)
	if !ok || branch != "automation/api-sync-${{ steps.release.outputs.version }}" {
		t.Fatalf("PR branch = %#v, want a contract-version-specific branch", openPR.With["branch"])
	}
	for _, step := range workflow.Jobs["sync"].Steps {
		if step.Name == "Queue generated PR for auto-merge" {
			t.Fatal("API sync workflow still auto-merges generated changes")
		}
	}
}

func TestAPISyncWorkflowTreatsOnlyOpenPullRequestsAsPending(t *testing.T) {
	workflow := readWorkflow(t, filepath.Join(testRepoRoot(t), ".github", "workflows", "api-sync.yml"))
	script := stepsByName(workflow.Jobs["sync"])["Fetch and verify exact Scalar contract"].Run

	tests := []struct {
		name         string
		openPR       string
		wantStatus   string
		wantGitCalls []string
		wantGoCalls  []string
	}{
		{
			name:       "closed PR with retained branch is proposed again",
			wantStatus: "status=new",
			wantGoCalls: []string{
				"run ./cmd/gen-endpoint candidate-status --lock contract.lock.json --spec PUBLISHED --version 1.2.3",
			},
		},
		{
			name:       "open PR with identical contract is pending",
			openPR:     "42",
			wantStatus: "status=current",
			wantGitCalls: []string{
				"fetch --no-tags origin refs/heads/automation/api-sync-1.2.3:refs/remotes/origin/automation/api-sync-1.2.3",
				"show refs/remotes/origin/automation/api-sync-1.2.3:contract.lock.json",
				"show refs/remotes/origin/automation/api-sync-1.2.3:spec.yaml",
			},
			wantGoCalls: []string{
				"run ./cmd/gen-endpoint candidate-status --lock contract.lock.json --spec PUBLISHED --version 1.2.3",
				"run ./cmd/gen-endpoint verify-lock --lock PENDING_LOCK --spec PENDING_SPEC",
				"run ./cmd/gen-endpoint candidate-status --lock PENDING_LOCK --spec PUBLISHED --version 1.2.3",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			temp := t.TempDir()
			output := filepath.Join(temp, "output")
			gitLog := filepath.Join(temp, "git.log")
			goLog := filepath.Join(temp, "go.log")
			runWorkflowScript(t, script, map[string]string{
				"CONTRACT_VERSION": "1.2.3",
				"EXPECTED_SHA256":  "",
				"GITHUB_OUTPUT":    output,
				"RUNNER_TEMP":      temp,
				"MOCK_OPEN_PR":     tt.openPR,
				"MOCK_GIT_LOG":     gitLog,
				"MOCK_GO_LOG":      goLog,
			}, map[string]string{
				"curl": `while [ "$#" -gt 0 ]; do if [ "$1" = "--output" ]; then shift; printf 'info: {version: 1.2.3}\n' > "$1"; exit 0; fi; shift; done`,
				"gh":   `printf '%s\n' "${MOCK_OPEN_PR}"`,
				"git":  `printf '%s\n' "$*" >> "${MOCK_GIT_LOG}"; if [ "$1" = "show" ]; then printf 'fixture\n'; fi`,
				"go": `printf '%s\n' "$*" | sed -e "s#${RUNNER_TEMP}/published.openapi.yaml#PUBLISHED#g" -e "s#${RUNNER_TEMP}/pending-contract.lock.json#PENDING_LOCK#g" -e "s#${RUNNER_TEMP}/pending-spec.yaml#PENDING_SPEC#g" >> "${MOCK_GO_LOG}"
count_file="${RUNNER_TEMP}/candidate-count"
if [ "$3" = "candidate-status" ]; then
  count=0; [ ! -f "${count_file}" ] || count="$(cat "${count_file}")"; count=$((count + 1)); printf '%s' "${count}" > "${count_file}"
  if [ "${count}" -eq 1 ]; then printf 'new\n'; else printf 'current\n'; fi
fi`,
			})

			if got := readLines(t, output)[0]; got != tt.wantStatus {
				t.Fatalf("status output = %q, want %q", got, tt.wantStatus)
			}
			if got := readLinesIfExists(t, gitLog); !reflect.DeepEqual(got, tt.wantGitCalls) {
				t.Fatalf("git calls = %#v, want %#v", got, tt.wantGitCalls)
			}
			if got := readLines(t, goLog); !reflect.DeepEqual(got, tt.wantGoCalls) {
				t.Fatalf("go calls = %#v, want %#v", got, tt.wantGoCalls)
			}
		})
	}
}

func TestAPISyncWorkflowVerifiesReviewableDrift(t *testing.T) {
	workflow := readWorkflow(t, filepath.Join(testRepoRoot(t), ".github", "workflows", "api-sync.yml"))
	script := stepsByName(workflow.Jobs["sync"])["Verify synchronized CLI"].Run
	temp := t.TempDir()
	goLog := filepath.Join(temp, "go.log")
	runWorkflowScript(t, script, map[string]string{
		"MOCK_GO_LOG":              goLog,
		"STRADDLE_API_SYNC_REVIEW": "",
	}, map[string]string{
		"go": `printf 'review=%s args=%s\n' "${STRADDLE_API_SYNC_REVIEW:-}" "$*" >> "${MOCK_GO_LOG}"`,
	})
	wantGoCalls := []string{
		"review= args=run ./cmd/gen-endpoint check --spec spec.yaml --repo . --agent",
		"review=true args=test ./...",
		"review= args=vet ./...",
	}

	if got := readLines(t, goLog); !reflect.DeepEqual(got, wantGoCalls) {
		t.Fatalf("go calls = %#v, want %#v", got, wantGoCalls)
	}
}

func TestMergedContractChangeTriggersExistingDistributionWorkflow(t *testing.T) {
	t.Parallel()

	repo := testRepoRoot(t)
	tagWorkflow := readWorkflow(t, filepath.Join(repo, ".github", "workflows", "api-sync-release.yml"))
	if got := tagWorkflow.On.Push.Branches; len(got) != 1 || got[0] != "main" {
		t.Fatalf("release handoff branches = %#v", got)
	}
	if got := tagWorkflow.On.Push.Paths; len(got) != 1 || got[0] != "contract.lock.json" {
		t.Fatalf("release handoff paths = %#v", got)
	}
	tagJob := tagWorkflow.Jobs["tag"]
	steps := stepsByName(tagJob)
	if steps["Create patch release tag"].If != "steps.release_changes.outputs.has_cli_changes == 'true' && steps.version.outputs.already_tagged != 'true'" {
		t.Fatalf("tag creation condition = %q", steps["Create patch release tag"].If)
	}
	temp := t.TempDir()
	gitLog := filepath.Join(temp, "git.log")
	runWorkflowScript(t, steps["Create patch release tag"].Run, map[string]string{
		"GITHUB_STEP_SUMMARY": filepath.Join(temp, "summary"),
		"MOCK_GIT_LOG":        gitLog,
		"RELEASE_SHA":         "abc123",
		"RELEASE_TAG":         "v1.2.4",
	}, map[string]string{"git": `printf '%s\n' "$*" >> "${MOCK_GIT_LOG}"`})
	wantGitCalls := []string{"tag v1.2.4 abc123", "push origin refs/tags/v1.2.4"}
	if got := readLines(t, gitLog); !reflect.DeepEqual(got, wantGitCalls) {
		t.Fatalf("git calls = %#v, want %#v", got, wantGitCalls)
	}

	releaseWorkflow := readWorkflow(t, filepath.Join(repo, ".github", "workflows", "release.yml"))
	if got := releaseWorkflow.On.Push.Tags; len(got) != 1 || got[0] != "v*" {
		t.Fatalf("release tag triggers = %#v", got)
	}
}

func TestPullRequestCIRejectsContractDowngrades(t *testing.T) {
	t.Parallel()

	workflow := readWorkflow(t, filepath.Join(testRepoRoot(t), ".github", "workflows", "ci.yml"))
	steps := stepsByName(workflow.Jobs["validate"])
	downgradeCheck := steps["Reject API contract downgrades"]
	if downgradeCheck.If != "github.event_name == 'pull_request'" {
		t.Fatalf("downgrade check condition = %q", downgradeCheck.If)
	}
	if downgradeCheck.Env["BASE_SHA"] != "${{ github.event.pull_request.base.sha }}" {
		t.Fatalf("downgrade check base SHA = %q", downgradeCheck.Env["BASE_SHA"])
	}
	temp := t.TempDir()
	goLog := filepath.Join(temp, "go.log")
	runWorkflowScript(t, downgradeCheck.Run, map[string]string{
		"BASE_SHA":    "base123",
		"RUNNER_TEMP": temp,
		"MOCK_GO_LOG": goLog,
	}, map[string]string{
		"git": `if [ "$1" = "cat-file" ]; then exit 0; fi; if [ "$1" = "show" ]; then printf 'fixture\n'; fi`,
		"go":  `printf '%s\n' "$*" | sed -e "s#${RUNNER_TEMP}/base-contract.lock.json#BASE_LOCK#g" -e "s#${RUNNER_TEMP}/base-spec.yaml#BASE_SPEC#g" >> "${MOCK_GO_LOG}"; if [ "$3" = "version" ]; then printf '1.2.3\n'; fi`,
	})
	wantGoCalls := []string{
		"run ./cmd/gen-endpoint verify-lock --lock BASE_LOCK --spec BASE_SPEC",
		"run ./cmd/gen-endpoint version --spec spec.yaml",
		"run ./cmd/gen-endpoint candidate-status --lock BASE_LOCK --spec spec.yaml --version 1.2.3",
	}
	if got := readLines(t, goLog); !reflect.DeepEqual(got, wantGoCalls) {
		t.Fatalf("go calls = %#v, want %#v", got, wantGoCalls)
	}
}

type workflowDocument struct {
	On struct {
		Schedule []struct {
			Cron string `json:"cron"`
		} `json:"schedule"`
		WorkflowDispatch struct {
			Inputs map[string]struct {
				Required bool   `json:"required"`
				Type     string `json:"type"`
			} `json:"inputs"`
		} `json:"workflow_dispatch"`
		RepositoryDispatch struct {
			Types []string `json:"types"`
		} `json:"repository_dispatch"`
		PullRequest struct {
			Types []string `json:"types"`
		} `json:"pull_request"`
		Push struct {
			Tags     []string `json:"tags"`
			Branches []string `json:"branches"`
			Paths    []string `json:"paths"`
		} `json:"push"`
	} `json:"on"`
	Jobs map[string]workflowJob `json:"jobs"`
}

type workflowJob struct {
	If    string         `json:"if"`
	Steps []workflowStep `json:"steps"`
}

type workflowStep struct {
	Name string            `json:"name"`
	If   string            `json:"if"`
	Uses string            `json:"uses"`
	Run  string            `json:"run"`
	Env  map[string]string `json:"env"`
	With map[string]any    `json:"with"`
}

func readWorkflow(t *testing.T, path string) workflowDocument {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read workflow %s: %v", path, err)
	}
	jsonData, err := yaml.YAMLToJSON(data)
	if err != nil {
		t.Fatalf("parse workflow %s: %v", path, err)
	}
	var workflow workflowDocument
	if err := json.Unmarshal(jsonData, &workflow); err != nil {
		t.Fatalf("decode workflow %s: %v", path, err)
	}
	return workflow
}

func stepsByName(job workflowJob) map[string]workflowStep {
	steps := make(map[string]workflowStep, len(job.Steps))
	for _, step := range job.Steps {
		steps[step.Name] = step
	}
	return steps
}

func runWorkflowScript(t *testing.T, script string, env map[string]string, stubs map[string]string) {
	t.Helper()
	binDir := filepath.Join(t.TempDir(), "bin")
	if err := os.Mkdir(binDir, 0o755); err != nil {
		t.Fatalf("create stub bin: %v", err)
	}
	for name, body := range stubs {
		path := filepath.Join(binDir, name)
		if err := os.WriteFile(path, []byte("#!/bin/bash\nset -euo pipefail\n"+body+"\n"), 0o755); err != nil {
			t.Fatalf("write %s stub: %v", name, err)
		}
	}
	cmd := exec.Command("bash", "-c", script)
	cmd.Dir = testRepoRoot(t)
	cmd.Env = append(os.Environ(), "PATH="+binDir+":"+os.Getenv("PATH"))
	for key, value := range env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("execute workflow step: %v\n%s", err, output)
	}
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

func readLinesIfExists(t *testing.T, path string) []string {
	t.Helper()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	return readLines(t, path)
}

func TestDriftSpecsReportsUnsupportedNonJSONRequestBodyAddition(t *testing.T) {
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

	result, err := apisync.DriftSpecs(baseSpec, headSpec)
	if err != nil {
		t.Fatalf("DriftSpecs: %v", err)
	}
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

func TestDriftSpecsReportsRequestBodyRefAsUnsupported(t *testing.T) {
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

	result, err := apisync.DriftSpecs(baseSpec, headSpec)
	if err != nil {
		t.Fatalf("DriftSpecs: %v", err)
	}
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

func TestUnsupportedReasonsRejectsJSONRequestBodyOnGet(t *testing.T) {
	t.Parallel()

	op := apisync.Operation{
		OperationID:           "ReadOperationWithBody",
		Method:                "GET",
		Path:                  "/v1/search",
		RequestBodyRequired:   true,
		RequestBodyMediaTypes: []string{"application/json"},
	}

	reasons := apisync.UnsupportedReasons(op)
	const want = "request body is not supported for GET operations"
	if len(reasons) != 1 || reasons[0] != want {
		t.Fatalf("UnsupportedReasons(GET with JSON body) = %#v, want [%q]", reasons, want)
	}
}

func TestUnsupportedReasonsAllowsJSONRequestBodyOnDelete(t *testing.T) {
	t.Parallel()

	op := apisync.Operation{
		OperationID:           "DeleteOperationWithBody",
		Method:                "DELETE",
		Path:                  "/v1/widgets",
		RequestBodyRequired:   true,
		RequestBodyMediaTypes: []string{"application/json"},
	}

	if reasons := apisync.UnsupportedReasons(op); len(reasons) != 0 {
		t.Fatalf("UnsupportedReasons(DELETE with JSON body) = %#v, want none", reasons)
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

func TestUnsupportedReasonsScopesArraySchemaTypeToQuery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		op            apisync.Operation
		wantReason    string
		wantSupported bool
	}{
		{
			name: "array path parameter is unsupported",
			op: apisync.Operation{
				OperationID: "ListWidgetsByTags",
				Endpoint:    "widgets.list-widgets-by-tags",
				Method:      "GET",
				Path:        "/v1/widgets/{tags}",
				PathParameters: []apisync.Parameter{
					{Name: "tags", In: "path", SchemaType: "array"},
				},
			},
			wantReason: `path parameter "tags" uses unsupported schema type array`,
		},
		{
			name: "array query parameter remains supported",
			op: apisync.Operation{
				OperationID: "ListWidgets",
				Endpoint:    "widgets.list-widgets",
				Method:      "GET",
				Path:        "/v1/widgets",
				QueryParameters: []apisync.Parameter{
					{Name: "status", In: "query", SchemaType: "array"},
				},
			},
			wantSupported: true,
		},
		{
			name: "array header parameter is unsupported at the gate",
			op: apisync.Operation{
				OperationID: "ListWidgets",
				Endpoint:    "widgets.list-widgets",
				Method:      "GET",
				Path:        "/v1/widgets",
				HeaderParameters: []apisync.Parameter{
					{Name: "X-Tags", In: "header", SchemaType: "array"},
				},
			},
			wantReason: `header parameter "X-Tags" uses unsupported schema type array`,
		},
		{
			name: "scalar string path parameter remains supported",
			op: apisync.Operation{
				OperationID: "GetWidget",
				Endpoint:    "widgets.get-widget",
				Method:      "GET",
				Path:        "/v1/widgets/{id}",
				PathParameters: []apisync.Parameter{
					{Name: "id", In: "path", SchemaType: "string"},
				},
			},
			wantSupported: true,
		},
		{
			name: "scalar integer query parameter remains supported",
			op: apisync.Operation{
				OperationID: "ListWidgets",
				Endpoint:    "widgets.list-widgets",
				Method:      "GET",
				Path:        "/v1/widgets",
				QueryParameters: []apisync.Parameter{
					{Name: "limit", In: "query", SchemaType: "integer"},
				},
			},
			wantSupported: true,
		},
		{
			name: "scalar string header parameter remains supported",
			op: apisync.Operation{
				OperationID: "ListWidgets",
				Endpoint:    "widgets.list-widgets",
				Method:      "GET",
				Path:        "/v1/widgets",
				HeaderParameters: []apisync.Parameter{
					{Name: "Request-Id", In: "header", SchemaType: "string"},
				},
			},
			wantSupported: true,
		},
		{
			name: "non-array non-scalar path parameter is unsupported",
			op: apisync.Operation{
				OperationID: "GetWidget",
				Endpoint:    "widgets.get-widget",
				Method:      "GET",
				Path:        "/v1/widgets/{filter}",
				PathParameters: []apisync.Parameter{
					{Name: "filter", In: "path", SchemaType: "object"},
				},
			},
			wantReason: `path parameter "filter" uses unsupported schema type object`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reasons := apisync.UnsupportedReasons(tc.op)
			if tc.wantSupported {
				if len(reasons) != 0 {
					t.Fatalf("UnsupportedReasons = %#v, want no reasons", reasons)
				}
				return
			}
			if !hasReasonContaining(reasons, tc.wantReason) {
				t.Fatalf("UnsupportedReasons = %#v, want reason containing %q", reasons, tc.wantReason)
			}
		})
	}
}

func TestDriftSpecsRoutesGeneratedParameterCollisionsToUnsupported(t *testing.T) {
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

	result, err := apisync.DriftSpecs(baseSpec, headSpec)
	if err != nil {
		t.Fatalf("DriftSpecs: %v", err)
	}
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

func TestDriftIgnoresPathParameterDescriptionOutsideSurface(t *testing.T) {
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

	result, err := apisync.DriftSpecs(baseSpec, headSpec)
	if err != nil {
		t.Fatalf("DriftSpecs: %v", err)
	}
	if !result.NoDrift {
		t.Fatalf("drift = %#v, want path parameter description ignored", result)
	}
	if len(result.Changes) != 0 {
		t.Fatalf("Changes = %d, want 0", len(result.Changes))
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

func TestUnsupportedReasonsAcceptsParameterizedJSONMediaType(t *testing.T) {
	op := apisync.Operation{OperationID: "CreateWidget", Method: "POST", Path: "/v1/widgets", RequestBodyRequired: true, RequestBodyMediaTypes: []string{"application/json; charset=utf-8"}}
	if reasons := apisync.UnsupportedReasons(op); len(reasons) != 0 {
		t.Fatalf("reasons = %#v", reasons)
	}
}
