# Operations

Local development commands, release process, and operational pointers for the Straddle CLI repo.

## Local development

| Task | Command |
|---|---|
| Build | `go build -o bin/straddle ./cmd/straddle` (or `make build`; never build to `/tmp`) |
| Test | `go test ./...` (or `make test`) |
| Vet | `go vet ./...` |
| Lint | `golangci-lint run` (or `make lint`) |
| Format | `gofmt -w <changed files>` (changed files only) |
| Endpoint coverage | `go run ./cmd/gen-endpoint check --spec spec.json --repo .` |
| Vulnerability scan | `make vuln` |
| Secret scan | `go run github.com/zricethezav/gitleaks/v8@latest detect --log-opts=--all` |
| Runtime smoke | `go run ./cmd/straddle doctor --json` and `go run ./cmd/straddle agent-context --pretty` |
| Install to PATH | `make install` (`go install ./cmd/straddle`) |

Agent mode: `--agent` = `--json --compact --no-input --no-color --yes`. Human color/rich output is opt-in via `--human-friendly`.

## CI

`.github/workflows/ci.yml` runs build, test, golangci-lint, govulncheck, and gitleaks (full history) on pushes to `main` and all PRs. PRs are additionally gated through the no-mistakes pipeline (`.no-mistakes.yaml`).

## API sync

`spec.json` is the OpenAPI lockfile. Drift and coverage tooling:

```bash
go run ./cmd/gen-endpoint check --spec spec.json --repo .
go run ./cmd/gen-endpoint drift --base spec.json --head <live-spec> --repo . --agent
go run ./cmd/gen-endpoint generate --spec <live-spec> --repo . --drift <drift-json> --supported-additions --agent
```

`.github/workflows/api-sync.yml` runs on a schedule, manual dispatch, and `repository_dispatch`. It fetches the live spec from `client_payload.spec_url`, a workflow input, or `STRADDLE_API_SPEC_URL`; opens PRs only for supported additions when `API_SYNC_BOT_TOKEN` is configured; and holds changed, removed, or unsupported operations for human review. Remote issue creation is opt-in via `API_SYNC_CREATE_ISSUES=true`.

## Release

Releases are cut from `main` by tag:

1. Push a `vX.Y.Z` tag.
2. `.github/workflows/release.yml` runs tests, then GoReleaser publishes the GitHub release (6 os/arch archives + `checksums.txt`) and publishes the `@straddleio/cli` npm wrapper (skipped automatically when `NPM_TOKEN` is unset). Homebrew cask upload is disabled by `homebrew_casks.skip_upload: true` in `.goreleaser.yaml`; adding `HOMEBREW_TAP_GITHUB_TOKEN` alone does not enable it.
3. `install.sh` and `go install github.com/straddle-build/cli/cmd/straddle@latest` resolve the new release with no further action.

Local dry run: `make release-snapshot` builds everything into `dist/` without publishing.

## Dependency maintenance

Dependabot (`.github/dependabot.yml`) runs weekly. Go module minor/patch updates are grouped as `go-minor-and-patch`, except `modernc.org/sqlite` — review SQLite updates separately because the local store depends on it. GitHub Actions updates are grouped together.

## Demo harness

`demo/` holds the VHS demo harness for marketing recordings (`spec.md`, `demo.tape.tmpl`, `make-demo.sh`, `demo-charge.sh`). Demo scripts assume specific CLI output; re-check them when changing output formatting.
