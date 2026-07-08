# Architecture and runtime surfaces

## Overview

This repo ships one binary:

- `straddle` — the interactive and scriptable user CLI

Human-friendly and agent-friendly modes are backed by the same Cobra
command tree and shared policy code, so humans and agents see the same
business rules.

## Entry points

- `cmd/straddle/main.go` calls `cli.Execute()`.
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

Agent integration happens through the CLI itself: `--agent` mode,
`agent-context` for runtime introspection, and the repo-root `SKILL.md`.
Commands carry `mcp:read-only` / `mcp:hidden` annotations as inert
metadata that `agent-context` surfaces so agents can detect read-only
commands. (An embedded MCP server was removed before the first release;
a standalone MCP server is a separate future project.)

The `api` command has two runtime surfaces:

- `straddle api` and `straddle api <interface>` browse hidden API interfaces.
- `straddle api <method> <path>` calls a raw API path for `GET`, `POST`, `PUT`, `PATCH`, and `DELETE`, using the same client, auth, account scoping, dry-run, verify, redaction, and output rules as the friendly commands.

Generated endpoint additions self-register through `internal/cli/generated_registry.go`, so a generated file can attach itself under an existing command family or a hidden parent command without hand-editing the root command.

## Important runtime rules

- `Straddle-Account-Id` scoping is centralized in `internal/straddleacct` for every request path.
- Endpoint annotations use `straddle:endpoint`, `straddle:method`, and `straddle:path`; `agent-context` schema version 4 exposes those keys.
- Output formatting must stay stable for agent/JSON use cases.
- The local store is expected by search, SQL, and analytics workflows.

## Where to look next

- [Domain map](../domain.md)
- [Source map](../source-map.md)
- `internal/cli/root.go`
