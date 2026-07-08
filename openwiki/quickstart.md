# OpenWiki Quickstart

## What this repository is

`straddle` is a Go CLI and MCP server for Straddle's Pay by Bank and Embed APIs. The repo combines:

- a human-facing terminal CLI (`straddle`)
- an MCP server for agents (`straddle-pp-mcp`)
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
- `cmd/straddle-pp-mcp/main.go` — MCP server entrypoint
- `internal/cli/` — Cobra command tree and hand-authored CLI features
- `internal/client/` — HTTP client and request helpers
- `internal/store/` — SQLite persistence and migrations
- `internal/mcp/` — MCP tool registration and execution
- `internal/straddleacct/` — integration-type rules for `Straddle-Account-Id`
- `demo/` — demo harness for marketing recordings and scripted walkthroughs

## What to know before changing code

- The repository is not regenerated from the OpenAPI spec anymore; it is maintained directly.
- Human CLI output and agent/JSON output are both intentional surfaces and should not drift casually.
- `Straddle-Account-Id` behavior is business-critical: it depends on the integration type (`account`, `saas`, `marketplace`) and the operation being called.
- The local SQLite store is part of the product, not a cache detail. Several commands and MCP tools assume it exists after `sync`.

## Best next pages

- Read [architecture/runtime.md](architecture/runtime.md) for the two binary surfaces and command plumbing.
- Read [domain.md](domain.md) for the main product concepts and how they map to commands.
- Read [operations.md](operations.md) before modifying sync, demo, or analytics behavior.
