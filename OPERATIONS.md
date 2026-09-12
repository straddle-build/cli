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
| Contract lock | `go run ./cmd/gen-endpoint verify-lock --spec spec.yaml` |
| Endpoint coverage | `go run ./cmd/gen-endpoint check --spec spec.yaml --repo .` |
| Contract mock | `STRADDLE_CONTRACT_MOCK=1 go test ./internal/cli -run TestContractMockServer -v` |
| Vulnerability scan | `make vuln` |
| Secret scan | `go run github.com/zricethezav/gitleaks/v8@latest detect --log-opts=--all` |
| Runtime smoke | `go run ./cmd/straddle doctor --json` and `go run ./cmd/straddle agent-context --pretty` |
| Install to PATH | `make install` (`go install ./cmd/straddle`) |

Agent mode: `--agent` = `--json --compact --no-input --no-color --yes`. Human color/rich output is opt-in via `--human-friendly`.

The contract mock check runs the registered `customers create --stdin --no-cache` command against Scalar using the `application/json` request example in `spec.yaml`. It prefers a media-type `example`, then the alphabetically first named media-type example, resolving local `#/components/examples/` references. When the media type has no example, it falls back to the referenced component schema's singular `example` or first plural `examples` value.

## CI

`.github/workflows/ci.yml` runs build, test, golangci-lint, govulncheck, and gitleaks (full history) on pushes to `main` and all PRs. PRs are additionally gated through the no-mistakes pipeline (`.no-mistakes.yaml`).

## API sync

`spec.yaml` contains the exact bytes of the immutable Scalar release named by `contract.lock.json`. The tooling derives each supported endpoint's command surface and compares drift field by field:

```bash
go run ./cmd/gen-endpoint verify-lock --spec spec.yaml
go run ./cmd/gen-endpoint surfaces --spec spec.yaml --agent
go run ./cmd/gen-endpoint check --spec spec.yaml --repo .
go run ./cmd/gen-endpoint drift --base spec.yaml --head <released-spec> --repo . --agent
go run ./cmd/gen-endpoint generate --spec spec.yaml --repo . --agent
```

`.github/workflows/api-sync.yml` receives `straddle-contract-published`, accepts an exact version for manual recovery, and checks Scalar daily for a missed event. Discovery may read Scalar's current release, but synchronization always downloads the exact versioned artifact. Publisher-triggered runs verify the publisher-provided digest before drift or generation. Scheduled and manual recovery runs compute the digest from the exact downloaded artifact and require no checksum input.

Every new contract version updates the YAML and lock in a normal human-reviewed PR. The workflow then regenerates every supported contract-derived endpoint file, overwrites existing generated files, and deletes owned generated files for operations that left the contract or became unsupported. Review evidence includes field-level flag additions, removals, and changes, plus counts for generated, deleted, unchanged, and unsupported operations. Repeated events and scheduled runs are green no-ops when the version and bytes already match `main` or the version-specific branch of an open synchronization PR. A stale branch without an open PR does not suppress PR creation. Changed bytes for an already-seen version fail. The workflow never auto-merges.

Configure `API_SYNC_BOT_TOKEN` with contents and pull-request write access. It creates the synchronization PR. After that PR merges, `.github/workflows/api-sync-release.yml` creates the next CLI patch tag only when the merge changed a file other than `spec.yaml` and `contract.lock.json`. A contract-only merge records the skipped release in the workflow summary. The existing release workflow publishes each new tag through every configured CLI distribution.

## Release

Releases are cut from `main` by tag. A merged version-specific `automation/api-sync-*` PR creates the next patch tag automatically; other releases may still be tagged manually.

1. Push a `vX.Y.Z` tag, or merge the generated contract synchronization PR.
2. `.github/workflows/release.yml` runs tests, then GoReleaser publishes the GitHub release (6 os/arch archives + `checksums.txt`) and publishes the `@straddlecom/cli` npm wrapper using npm trusted publishing. An npm publication failure fails the workflow. GoReleaser publishes the Homebrew cask when `HOMEBREW_TAP_GITHUB_TOKEN` is configured.
3. `install.sh` and `go install github.com/straddle-build/straddle-cli/cmd/straddle@latest` resolve the new release with no further action.

Local dry run: `make release-snapshot` builds everything into `dist/` without publishing.

### npm setup (once per package)

The CLI is `@straddlecom/cli`; `@straddlecom/straddle` remains the TypeScript SDK. npm permissions belong to each package, so the SDK's trusted-publisher connection does not authorize CLI releases.

Before the first automated CLI release, an npm maintainer must publish `@straddlecom/cli@1.0.2` from `npm/`, aligned with Scalar contract 1.0.2. Do not bootstrap 0.1.1, any other version, or the placeholder `0.0.0`. Then configure the CLI package's Settings > Trusted Publisher for GitHub Actions:

- Organization: `straddle-build`
- Repository: `straddle-cli`
- Workflow filename: `release.yml`
- Environment: leave blank (the release job does not use a GitHub environment)

Allow `npm publish` in that connection. Subsequent CLI releases authenticate through GitHub OIDC without an `NPM_TOKEN`. See [npm trusted publishing](https://docs.npmjs.com/trusted-publishers/). Verify an actual automated publication before treating npm delivery as connected.

## Dependency maintenance

Dependabot (`.github/dependabot.yml`) runs weekly. Go module minor/patch updates are grouped as `go-minor-and-patch`, except `modernc.org/sqlite` — review SQLite updates separately because the local store depends on it. GitHub Actions updates are grouped together.

## Demo harness

`demo/` holds the VHS demo harness for marketing recordings (`spec.md`, `demo.tape.tmpl`, `make-demo.sh`, `demo-charge.sh`). Demo scripts assume specific CLI output; re-check them when changing output formatting.
