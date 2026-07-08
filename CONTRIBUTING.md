# Contributing

## Setup

Install the Go version pinned in [`go.mod`](go.mod), then:

```bash
make build             # build bin/straddle
make test              # go test ./...
make lint              # golangci-lint run (config: .golangci.yml)
make release-snapshot  # local GoReleaser dry run into dist/
```

## How this repo is maintained

This repo is maintained as a standalone Go CLI. Edit source directly, keep changes narrow, and preserve the human and agent output contracts unless a change explicitly requires them to move.

`cmd/gen-endpoint` owns endpoint coverage, drift classification, and generic endpoint generation from `spec.json`. Run `go run ./cmd/gen-endpoint check --spec spec.json --repo .` when command annotations or the API lockfile change.

Dependabot runs weekly for Go modules and GitHub Actions. Go module minor and patch updates are grouped under `go-minor-and-patch`, except `modernc.org/sqlite`, which stays out of that group because the local SQLite store is operationally sensitive. GitHub Actions updates are grouped together.

The repo-local `.no-mistakes.yaml` pins gate commands: test runs `go build ./... && go test ./...`; lint runs `go vet ./...`, `golangci-lint`, `govulncheck`, and `gitleaks`.

## Security

See [SECURITY.md](SECURITY.md). Never include real credentials or
customer data in tests, fixtures, or commit history.
