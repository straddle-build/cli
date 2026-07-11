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

Per the root `OPERATIONS.md` local development table, the expected validation loop is:

- `go test ./...`
- `go vet ./...`
- `go run ./cmd/gen-endpoint check --spec spec.json --repo .`
- `go build -o ./straddle ./cmd/straddle`
- `./straddle doctor`

There are package-level tests around CLI behavior, store migrations, account scoping, and the special output/rendering logic.

## API sync workflow

Endpoint coverage and drift are maintained by `cmd/gen-endpoint` and `.github/workflows/api-sync.yml`.

Local commands:

- `go run ./cmd/gen-endpoint check --spec spec.json --repo .` checks that `straddle:*` endpoint annotations cover the OpenAPI lockfile.
- `go run ./cmd/gen-endpoint drift --base spec.json --head <live-spec> --repo . --agent` classifies supported additions, changed operations, removed operations, and unsupported operation shapes.
- `go run ./cmd/gen-endpoint generate --spec <live-spec> --repo . --drift <drift-json> --supported-additions --agent` writes deterministic generic endpoint command files for supported additions.

The GitHub workflow runs on a schedule, manual dispatch, and `repository_dispatch` events. It fetches the live spec from `client_payload.spec_url`, a workflow input, or `STRADDLE_API_SPEC_URL`; opens PRs only for supported additions when `API_SYNC_BOT_TOKEN` is configured; and holds changed, removed, or unsupported operations for human review. Remote issue creation is opt-in with `API_SYNC_CREATE_ISSUES=true`.

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
