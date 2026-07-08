# Architecture and runtime surfaces

## Overview

This repo exposes Straddle through two binaries:

- `straddle-pp-cli` — the interactive and scriptable user CLI
- `straddle-pp-mcp` — the Model Context Protocol server for agent use

Both surfaces are backed by the same Cobra command tree and shared policy code, so humans and agents see the same business rules.

## Entry points

- `cmd/straddle-pp-cli/main.go` calls `cli.Execute()`.
- `cmd/straddle-pp-mcp/main.go` starts an MCP server and registers tools from `internal/mcp`.
- `internal/cli/root.go` defines the root Cobra command, persistent flags, usage/error handling, and root execution path.

## CLI surface

The root command supports both human-friendly and agent-friendly output modes. Notable root flags include:

- `--json`
- `--compact`
- `--agent`
- `--human-friendly`
- `--no-input`
- `--yes`
- `--deliver`
- `--profile`
- `--data-source`
- `--account`

The root command is deliberately non-interactive in execution mode; prompts are avoided so agents and scripts behave predictably.

## MCP surface

`internal/mcp/tools.go` registers several categories of tools:

- `search` for full-text search over synced local data
- `sql` for read-only analysis over the SQLite store
- `context` for domain guidance to agents
- generated Cobra-tree mirrors of the CLI command surface

`cmd/straddle-pp-mcp/main.go` can run either over `stdio` or streamable HTTP. The transport defaults to `stdio`, with `PP_MCP_TRANSPORT` as an environment override.

## Shared command model

The MCP layer mirrors the CLI tree instead of inventing a separate tool schema. That matters because the repo wants humans and agents to execute the same business operations with the same validation and scoping rules.

## Important runtime rules

- `Straddle-Account-Id` scoping is centralized in `internal/straddleacct` and is used by both CLI and MCP request paths.
- Output formatting must stay stable for agent/JSON use cases.
- The local store is expected by search, SQL, and analytics workflows.

## Where to look next

- [Domain map](../domain.md)
- [Source map](../source-map.md)
- `internal/mcp/`
- `internal/cli/root.go`
