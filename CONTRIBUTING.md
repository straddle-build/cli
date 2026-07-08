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

## Security

See [SECURITY.md](SECURITY.md). Never include real credentials or
customer data in tests, fixtures, or commit history.
