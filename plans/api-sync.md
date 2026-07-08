# Straddle CLI API Sync Plan

## Source of truth

- Linear: ME-343, `api-sync: generator-backed endpoint tracking + straddle api passthrough`.
- Companion Linear: ME-344, Scalar registry publish + repository dispatch hook.
- Repo: `github.com/straddle-build/cli`.
- Current OpenAPI lockfile: `spec.json`.

## Decisions carried forward

- Drift scope is endpoint-level only. Field-level changes are handled by `--stdin` and by typed-command maintenance later.
- `straddle api <method> <path>` is the immediate escape hatch for newly published endpoints.
- `spec.json` is the living API lockfile in this repo.
- Generated code and sync automation use Straddle-owned names only. The old Printing Press manifests were intentionally removed and are not revived.
- "Captured" means merged to `main`; release tags remain a human action.
- Bot-created PRs that need normal PR CI must use a fine-grained PAT or GitHub App token, not the default `GITHUB_TOKEN`.

## Implementation slices

1. Generator foundation
   - Add a repo-local `gen-endpoint` tool with a stable CLI for generation, checking, and drift classification.
   - Parse OpenAPI 3.1 operations from `spec.json` and ignore webhook-only Svix event operations.
   - Use existing `straddle:*` command annotations as the current command inventory.
   - Golden-test current endpoint coverage and generated output against the checked-in command tree.
   - Current generator gap: `generate` emits deterministic generic endpoint scaffolds for supported additions (path/query flags plus `--stdin` JSON bodies) and self-registers them through the generated endpoint registry. It does not byte-for-byte reproduce the 70 checked-in hand-tuned command files, their typed body flags, or bespoke validation. The delivered generator contract for this slice is parse/inventory/coverage/check/drift/generic generation plus explicit unsupported-shape classification.

2. Raw API passthrough
   - Extend the existing `api` command without breaking discovery mode.
   - Treat a first positional token matching an HTTP verb as raw passthrough: `straddle api <method> <path>`.
   - Support query params with repeatable `--param key=value` and JSON bodies with `--stdin`.
   - Reuse the existing client so auth, redirect hardening, verify short-circuiting, dry-run, redaction, and output filtering stay centralized.

3. Drift automation
   - Add `.github/workflows/api-sync.yml` with `schedule`, `workflow_dispatch`, and `repository_dispatch` triggers.
   - Fetch the live spec into a temp file, normalize it, classify drift against `spec.json`, and run the generator.
   - Open PRs only for supported additions when no changed, removed, or unsupported operations are present.
   - Hold changed, removed, and unsupported operations for human review. Remote issue creation is opt-in until ME-344 finalizes dispatch and dedupe policy.
   - Leave the source URL as the interim Stainless artifact until ME-344 swaps it to Scalar.

4. Companion follow-up
   - Record the exact Scalar URL and dispatch payload expected from ME-344 once that external publishing path exists.
   - Keep the CLI workflow runnable by cron before the dispatch hook exists.

## ME-344 handoff assumptions

- ME-344 owns the external Scalar publishing path. This repo only prepares the receiving workflow.
- ME-344 must provide the blessed HTTPS OpenAPI URL from Scalar as `client_payload.spec_url` on dispatch. Until then, the workflow may keep using the interim Stainless source through `STRADDLE_API_SPEC_URL` as a repository variable or secret.
- The primary `repository_dispatch` event type is `straddle-api-spec-published`. The CLI workflow may also accept `api-sync` for manual integrations.
- Dispatch payload shape: `client_payload.spec_url` is required for event-driven runs; `client_payload.source`, `client_payload.spec_sha`, and `client_payload.published_at` are optional provenance fields.
- The CLI workflow must use `API_SYNC_BOT_TOKEN` for PR or issue mutation so generated PRs trigger normal CI. It must not rely on `GITHUB_TOKEN` for bot PR automation. Any token used by the external publisher to call `repository_dispatch` lives outside this repo.

## Verification targets

- `go test ./internal/cli ./internal/client` for passthrough and client behavior.
- `go test ./cmd/gen-endpoint ./internal/apisync` for generator and drift logic.
- `go test ./...` before committing the full slice.
- `go run ./cmd/straddle api --agent` must preserve discovery output.
- `STRADDLE_VERIFY=1 go run ./cmd/straddle api post /v1/charges --stdin --agent` must return a verify no-op envelope without dialing.
