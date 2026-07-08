# Straddle CLI Agent Guide

This repository is the Straddle CLI (`straddle`), a standalone Go CLI maintained and released directly from this repo. Keep edits narrow and use runtime discovery for current behavior.

## OpenWiki

This repository has documentation located in the /openwiki directory.

Start here:
- [OpenWiki quickstart](openwiki/quickstart.md)

OpenWiki includes repository overview, architecture notes, workflows, domain concepts, operations, integrations, testing guidance, and source maps.

When working in this repository, read the OpenWiki quickstart first, then follow its links to the relevant architecture, workflow, domain, operation, and testing notes.

## Local Operating Contract

Start by asking the working-tree CLI for current runtime truth. Do not use a bare `straddle` from PATH for repo validation unless you have just rebuilt and verified that binary from this checkout:

```bash
go run ./cmd/straddle doctor --json
go run ./cmd/straddle agent-context --pretty
```

Use runtime discovery instead of relying on a copied command list:

```bash
go run ./cmd/straddle which "<capability>" --json
go run ./cmd/straddle <command> --help
```

Add `--agent` to command invocations for JSON, compact output, non-interactive defaults, no color, and confirmation-safe scripting:

```bash
go run ./cmd/straddle <command> --agent
```

Before running an unfamiliar command that may mutate remote state, inspect its help and prefer a dry run:

```bash
go run ./cmd/straddle <command> --help
go run ./cmd/straddle <command> --dry-run --agent
```

Use `--yes --no-input` only after the target, arguments, and side effects are clear.

For install, auth, examples, and longer product guidance, read `README.md` and `SKILL.md`. This file intentionally stays small so repo-local agents get invariant local guidance without duplicating broader docs.
