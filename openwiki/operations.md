# Operations, testing, and demo workflows

## Everyday operator flow

The README and command tree imply a practical workflow:

1. verify credentials and connectivity with `doctor`
2. sync the local store
3. use search, payments, reconciliation, and analytics commands against synced data
4. use account-scoping commands when working in Embed/SaaS/marketplace contexts

## Commands that matter operationally

Hand-authored commands carry most of the operational value in this repo:

- `doctor` — checks auth/connectivity
- `sync` — populates the local store
- `search` — finds synced records
- `payments` — unified payments view
- `reconcile` — matches payments to funding events
- `pipeline` — identifies cancelable vs locked payments
- `returns` — surfaces return/reversal analysis
- `review-queue` — triages KYC/KYB review backlog
- `expiring` — finds paykeys that need attention
- `sandbox` — exposes deterministic sandbox outcomes
- `setup` / `use-account` — persist integration type and active account scope

## Demo harness

The `demo/` directory was recently added to support marketing recordings and scripted demonstrations. The current artifacts include:

- `demo/spec.md`
- `demo/tape.tmpl`
- `demo/make-demo.sh`
- `demo/demo-charge.sh`

The git history shows `demo-charge.sh` was ported from the v1 CLI and the VHS demo harness was added in the preceding commit.

## Testing and validation

From `CLAUDE.md`, the expected validation loop is:

- `go test ./...`
- `go vet ./...`
- `go build -o ./straddle-pp-cli ./cmd/straddle-pp-cli`
- `./straddle-pp-cli doctor`

There are package-level tests around CLI behavior, store migrations, MCP tooling, account scoping, and the special output/rendering logic.

## Change warnings

Be careful when changing any of these areas:

- human/agent output formatting
- account scoping rules
- migration behavior in `internal/store`
- command execution side effects
- demo scripts that assume specific CLI output

## Useful files

- `README.md`
- `CLAUDE.md`
- `SKILL.md`
- `demo/`
- `internal/cli/root.go`
- `internal/cli/straddle_setup.go`
- `internal/store/store.go`
- `internal/mcp/tools.go`
