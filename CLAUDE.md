# CLAUDE.md — Straddle CLI (`straddle`)

## What this is
A standalone Go CLI for Straddle's **Pay by Bank + Embed** APIs, with a human surface and
an agent surface (`--agent`) from one binary. It also keeps a local SQLite mirror of
synced resources and adds settlement/return analytics the official `straddle` CLI lacks.

- Module: `github.com/straddle-build/cli` (Go + Cobra). Binary name: `straddle`.
- The command tree, local store, workflows, docs, and release config are maintained
  directly in this repo. Treat this repo as the source of truth for CLI behavior.
- The embedded MCP server was removed before the first release (git history keeps it);
  a standalone MCP server is a separate future project.

## OpenWiki

This repository has documentation located in the /openwiki directory.

Start here:
- [OpenWiki quickstart](openwiki/quickstart.md)

OpenWiki includes repository overview, architecture notes, workflows, domain concepts, operations, integrations, testing guidance, and source maps.

When working in this repository, read the OpenWiki quickstart first, then follow its links to the relevant architecture, workflow, domain, operation, and testing notes.

## Build / test / run
```bash
go build -o ./straddle ./cmd/straddle   # always build to ./, never /tmp
go test ./...                                          # must be green before done
go vet ./...
gofmt -w <changed files>                               # don't gofmt the whole tree blindly
./straddle --help
./straddle doctor                               # verify auth/connectivity
```
- Agent mode: `--agent` (= `--json --compact --no-input --no-color --yes`).
- Human color/rich: `--human-friendly` (off by default — agent-safe).

## Layout
- `cmd/straddle/` — entry point
- `internal/cli/` — Cobra commands (one file per endpoint) + hand-authored novel commands (`straddle_*.go`)
- `internal/client/` — HTTP client; applies `Config.Headers` to every request
- `internal/config/` — `config.toml` load/save
- `internal/store/` — SQLite store: typed tables (spec-derived columns) + generic `resources` fallback
- `internal/cliutil/` — shared CLI helpers (sanitization, rate limiting, env probes)
- `internal/straddleacct/` — integration-type + `Straddle-Account-Id` gating (see below)
- `spec.json` — the OpenAPI spec; **authoritative source for resource/response shapes**

## Key invariant: `Straddle-Account-Id` scoping (hand-authored — don't break it)
`internal/straddleacct/` decides when the header is sent, by integration type:
- `setup --type account|saas|marketplace` — set the type (sticky, in `platform.toml`).
- `use-account <id>` — set the acting account (sticky); `use-account --clear` to unset.
- `--account <id>` — per-call override on any command.
- Policy (47/23 split derived from the spec): charges/payouts create → **required** for `saas`+`marketplace`; customers/paykeys/bridge → scoped for `saas`, **no header** for `marketplace` (platform owns); account-management ops (accounts/orgs/representatives/linked-banks/onboarding) **never** use the header (they carry the account in the body/path).
- Both the CLI pre-run and the in-process MCP handler call `straddleacct`, so agents and humans are gated identically. Route any new account-scoped behavior through this package.

## Output rules
- **Agent/JSON output must stay byte-stable.** Never change it for cosmetics.
- Human mode today: opt-in color + `tabwriter` tables (`printAutoTable`); single-object `get`
  prints JSON/raw. There is **no framed "detail card" renderer yet** (a planned feature).
- Per-response field curation already exists: `compactFields` (`internal/cli/helpers.go`) and
  the typed store columns (`internal/store/store.go`). Per-resource shapes come from `spec.json`.

## Domain help (use when building features)
- Skills: `/straddle:straddle-integrate`, `/straddle:straddle-setup` — entities, paykey
  lifecycle, payment status machine, the account-id rules.
- Docs: https://docs.straddle.com (append `.md` to any page for raw markdown; full index at
  https://docs.straddle.com/llms.txt). Spec: `api-reference/openapi.json`.
- Bundled `SKILL.md` teaches agents how to use this CLI's commands. `AGENTS.md`
  and `README.md` are repo-local usage guides.

## Maintenance model
- `spec.json` captures the OpenAPI resource and response shapes the CLI supports.
- Endpoint command files, store schemas, account-scoping policy, analytics workflows,
  and docs are maintained directly in this repo.
- Product commands include `reconcile`, `pipeline`, `returns`, `review-queue`,
  `cashflow`, `expiring`, `sandbox`, `sql`, `setup`, and `use-account`.
- Preserve the dual human and agent surfaces when changing commands; update tests
  and docs with the code.

## Conventions
- TDD for non-trivial logic; table-driven tests, stdlib `testing`. Run `go test ./...` before done.
- Add only deps you need. Keep the dual human/agent surface intact.
