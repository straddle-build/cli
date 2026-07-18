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
- `api` - browses hidden API interfaces and calls raw API paths

## Demo harness

The `demo/` directory was recently added to support marketing recordings and scripted demonstrations. The current artifacts include:

- `demo/spec.md`
- `demo/tape.tmpl`
- `demo/make-demo.sh`
- `demo/demo-charge.sh`

The git history shows `demo-charge.sh` was ported from the v1 CLI and the VHS demo harness was added in the preceding commit.

## Testing and validation

Use the [root local development table](../OPERATIONS.md#local-development) as the single source of truth for validation commands.

There are package-level tests around CLI behavior, store migrations, account scoping, and the special output/rendering logic.

## API sync workflow

See the [root API sync operations section](../OPERATIONS.md#api-sync) for the canonical commands, workflow behavior, remote-mutation guards, auto-merge queueing, and review routing.

## Change warnings

Be careful when changing any of these areas:

- human/agent output formatting
- account scoping rules
- migration behavior in `internal/store`
- command execution side effects
- raw `api` passthrough behavior
- API sync drift classification
- demo scripts that assume specific CLI output

## Useful files

- `README.md`
- `AGENTS.md`
- `OPERATIONS.md`
- `SKILL.md`
- `demo/`
- `cmd/gen-endpoint/`
- `internal/apisync/`
- `.github/workflows/api-sync.yml`
- `.github/dependabot.yml`
- `internal/cli/root.go`
- `internal/cli/straddle_setup.go`
- `internal/store/store.go`

## Release process

See the root `OPERATIONS.md` for the canonical release procedure and current publish targets.

## Dependency maintenance

Dependabot is configured in `.github/dependabot.yml` for weekly Go module and GitHub Actions updates. Go module minor and patch updates are grouped under `go-minor-and-patch`, except `modernc.org/sqlite`; review SQLite updates separately because the local store depends on it. GitHub Actions updates are grouped together.
