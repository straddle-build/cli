# OpenWiki Quickstart

## What this repository is

`straddle` is a Go CLI for Straddle's Pay by Bank and Embed APIs. The repo combines:

- a human-facing terminal CLI (`straddle`) with an agent mode (`--agent`)
- a local SQLite mirror for synced Straddle resources
- local analytics and workflows that go beyond one-off API calls

The project is centered on payment operations: charges, payouts, customers, paykeys, linked bank accounts, organizations, representatives, funding events, and account-scoped platform workflows.

## Start here

- [Architecture and runtime surfaces](architecture/runtime.md)
- [Domain map: payments, identity, and platform scoping](domain.md)
- [Local store and sync model](data-model.md)
- [Operations, testing, and demo workflows](operations.md)
- [Source map for key packages](source-map.md)

## Repository shape

The main entrypoints and packages are:

- `cmd/straddle/main.go` — CLI entrypoint
- `cmd/gen-endpoint/` - API sync generator, endpoint coverage checker, and drift classifier
- `internal/cli/` — Cobra command tree and hand-authored CLI features
- `internal/apisync/` - OpenAPI parsing, repo inventory, drift classification, and endpoint generation support
- `internal/client/` — HTTP client and request helpers
- `internal/store/` — SQLite persistence and migrations
- `internal/straddleacct/` — integration-type rules for `Straddle-Account-Id`
- `demo/` — demo harness for marketing recordings and scripted walkthroughs

## What to know before changing code

- The repository is maintained as a standalone Go CLI with source, docs, and release config in this repo.
- `spec.json` is the API lockfile; `cmd/gen-endpoint` keeps endpoint coverage and drift visible.
- Human CLI output and agent/JSON output are both intentional surfaces and should not drift casually.
- `Straddle-Account-Id` behavior is business-critical: it depends on the integration type (`account`, `saas`, `marketplace`) and the operation being called.
- The local SQLite store is part of the product, not a cache detail. Several commands assume it exists after `sync`.

## Best next pages

- Read [architecture/runtime.md](architecture/runtime.md) for the two binary surfaces and command plumbing.
- Read [domain.md](domain.md) for the main product concepts and how they map to commands.
- Read [operations.md](operations.md) before modifying sync, demo, or analytics behavior.
