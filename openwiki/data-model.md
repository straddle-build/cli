# Local store and sync model

## Overview

The CLI keeps a local SQLite store so it can search, reconcile, and analyze Straddle data offline. This is a major product feature, not just a cache.

The store lives in `internal/store/` and is opened through `internal/store/store.go`.

## What the store is used for

The local database supports:

- search across synced resources
- analytics commands such as reconciliation and cashflow reporting
- the `search` and `sql` commands
- offline read paths when live API access is unavailable or unnecessary

## Storage characteristics

Key implementation details from `internal/store/store.go`:

- it uses `modernc.org/sqlite`, so the repo stays pure Go and cross-compilable
- the database runs in WAL mode
- write access is serialized with a mutex
- a `resources_fts` FTS5 virtual table provides full-text search
- the schema version is tracked with `PRAGMA user_version`

## Schema evolution

The store includes migration and backfill logic so older databases can be upgraded in place. The code explicitly handles added columns that newer binaries expect.

That means changes to the store are operationally sensitive:

- adding/removing columns affects migrations
- typed resource tables and fallback resource storage both matter
- search and analytics code may depend on those tables being populated

## Sync model

Although sync orchestration spans more than one file, the overall pattern is straightforward:

1. authenticate and connect to Straddle
2. fetch resources from the API
3. upsert them into the local database
4. use the synced store for subsequent search/analytics commands

If the store is empty, commands like search or reconciliation will not be useful until sync runs.

## Where to start in code

- `internal/store/store.go`
- `internal/cli/sync.go`
- `internal/cli/search.go`
- `internal/cli/straddle_reconcile.go`
- `internal/cli/straddle_cashflow.go`
