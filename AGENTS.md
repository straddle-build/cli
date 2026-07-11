# Straddle CLI

This repository is the Straddle CLI (`straddle`): a standalone Go CLI for Straddle's Pay by Bank and Embed APIs, with a human surface and an agent surface (`--agent`) from one binary, plus a local SQLite mirror and settlement/return analytics. Source, docs, and release config live in this repo; treat it as the source of truth for CLI behavior. `CLAUDE.md` is a compatibility symlink to this file.

**MANDATORY** Before implementation work, agents MUST read and MUST follow both repo-root standards, `STRADDLE_STYLE.MD` and `CODING_STANDARDS.MD`.

This repository has OpenWiki documentation in `/openwiki`. When working here, read `openwiki/quickstart.md` first, then follow its links to the relevant architecture, workflow, domain, operation, and testing notes.

## Instruction precedence

1. System and user instructions.
2. The nearest AGENTS.md to the files being changed.
3. This root AGENTS.md.
4. Source code, schemas, and CLI help output.

Pointers: `OPERATIONS.md` (local dev commands, release, API sync), `openwiki/quickstart.md` (architecture, domain, source maps), `README.md` and `SKILL.md` (install, auth, product usage).

## Working in this repo

- Read `openwiki/domain.md` and `openwiki/data-model.md` before changing domain behavior (payments, paykeys, account scoping, the local store).
- Run the applicable verification commands from the `OPERATIONS.md` local development table before claiming completion.
- Prefer runtime discovery over copied command lists: `go run ./cmd/straddle which "<capability>" --json`, `go run ./cmd/straddle <command> --help`. Do not validate against a bare `straddle` from PATH unless you just rebuilt and installed it from this checkout.
- Before running an unfamiliar command that may mutate remote state, inspect its help and prefer `--dry-run --agent`. Use `--yes --no-input` only after the target, arguments, and side effects are clear.

## Project invariants

- **`Straddle-Account-Id` scoping is business-critical.** `internal/straddleacct/` decides when the header is sent, by integration type (`account`, `saas`, `marketplace`): charges/payouts create require it for `saas`+`marketplace`; customers/paykeys/bridge are scoped for `saas` and send no header for `marketplace`; account-management ops never use the header. The CLI pre-run gates agents and humans identically. Route any new account-scoped behavior through this package.
- **Agent/JSON output must stay byte-stable.** Never change it for cosmetics. The human and agent surfaces are both intentional; preserve both when changing commands, and update tests and docs with the code.
- `spec.json` is the OpenAPI lockfile and the authoritative source for resource and response shapes. Keep endpoint coverage and drift visible via `cmd/gen-endpoint` (see `OPERATIONS.md`).
- The local SQLite store (`internal/store/`) is part of the product, not a cache detail. Several commands assume it exists after `sync`. Be careful with migration behavior.
- TDD for non-trivial logic; table-driven tests with stdlib `testing`. `go test ./...` must be green before done.
- Add only dependencies you need. Build to the repo (`bin/` or `./`), never `/tmp`. Don't `gofmt` the whole tree blindly; format only changed files.

## Security

- Never commit credentials. API keys live in `config.toml` (managed by `straddle setup`/`auth`), not in the repo. CI runs gitleaks over full history; `.gitleaks.toml` holds the allowlist.
- Release secrets (`HOMEBREW_TAP_GITHUB_TOKEN`, `NPM_TOKEN`, `API_SYNC_BOT_TOKEN`) exist only as GitHub Actions secrets.

## Generated code boundary

Generated endpoint command files self-register through `internal/cli/generated_registry.go` and are produced deterministically by `cmd/gen-endpoint generate`. Do not hand-edit generated endpoint files; change the generator or the hand-authored `straddle_*.go` commands instead. Raw `straddle api <method> <path>` is the fallback for newly published endpoints before a dedicated command exists.
