# Security Policy

## Supported versions

Only the latest release receives security fixes. Update to the newest
version before reporting an issue you can still reproduce.

## Reporting a vulnerability

Report vulnerabilities through Straddle's responsible disclosure program
at **https://trust.straddle.com/** — never through public GitHub issues,
pull requests, or discussions.

Do not include secrets, API keys, or customer PII in reports. Redact
tokens and account identifiers from logs and proofs of concept.

## Local data

The CLI stores synced API data and cached responses on your machine:

- `~/.local/share/straddle/` — local SQLite store (synced charges,
  customers, paykeys, funding events)
- `~/.cache/straddle/` — HTTP response cache
- `~/.config/straddle/config.toml` — configuration, including saved
  credentials

All three are written owner-only (directories `0700`, files `0600`).
Treat them as sensitive: they can contain customer PII and payment data.
`straddle auth logout` clears saved credentials.
