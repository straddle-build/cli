# Spec: Auto-Running CLI Demo for Marketing Recording

## Objective

A hands-free, re-renderable demo of `straddle-pp-cli` for product marketing.
A VHS tape types a ~90-second highlight reel into a real terminal with natural
pacing and renders it to video — no human at the keyboard, repeatable after
every CLI change.

Audience: prospects/developers evaluating Straddle tooling. Success looks
like: one command (`demo/make-demo.sh`) produces a polished MP4 + GIF showing
the CLI's best moments, every take identical in structure, no 404s, no stale
data, no visible glitches.

## Demo Narrative (the ~90s reel)

| Beat | Command (as typed on screen) | What it shows |
|---|---|---|
| 1 | `straddle-pp-cli doctor` | Instant health check: auth, connectivity, local store |
| 2 | `straddle-pp-cli sync --resources customers,payments,funding-events` | Live API → local SQLite mirror |
| 3 | `straddle-pp-cli customers get <fresh-id> --human-friendly` | The detail card renderer (branch feature) |
| 4 | `straddle-pp-cli reconcile` | Settled vs outstanding — analytics the official CLI lacks |
| 5 | `straddle-pp-cli pipeline` | Lifecycle status table + cancelable-now summary (swapped in for `returns`, which is monotone in this sandbox: all `insufficient_funds`, no reversals) |
| 6 | `straddle-pp-cli sql "select status, count(*) n from payments group by status order by n desc"` | Your payments data is just SQL |

`<fresh-id>` is a real UUID substituted at render time (see Structure) so the
screen shows an honest-looking command, and it can never 404.

## Tech Stack

- charmbracelet **VHS** (new dep, `brew install vhs`; brings `ttyd` + `ffmpeg`)
- bash for the prepare/render wrapper
- The CLI itself, built from the current branch (`go build -o ./straddle-pp-cli ./cmd/straddle-pp-cli`)
- Sandbox environment via the existing oauth2 config — no new credentials

## Commands

```bash
brew install vhs                      # one-time
./demo/make-demo.sh                   # sync → pick ids → render tape → demo/out/demo.mp4 + demo.gif
vhs demo/out/demo.tape                # re-render only (after make-demo.sh has run once)
```

## Project Structure

```
demo/
  spec.md            → this document
  demo.tape.tmpl     → VHS tape template with {{CUSTOMER_ID}}-style placeholders
  make-demo.sh       → pre-flight: build CLI, sync, resolve fresh ids from the
                       local store, substitute into demo/out/demo.tape, run vhs
  out/               → generated tape + demo.mp4 + demo.gif (gitignored)
```

The tape is generated, not hand-edited; the template is the source of truth.

## Code Style

Plain defensive bash, matching the repo's simple-tooling bias:

```bash
#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

CUSTOMER_ID=$(./straddle-pp-cli sql \
  "select id from customers order by created_at desc limit 1" --quiet)
[ -n "$CUSTOMER_ID" ] || { echo "no customers in local store" >&2; exit 1; }
```

Tape style: `Set Theme`, ~110x30 terminal, readable font size (≥ 22), typed at
human speed (`Set TypingSpeed 75ms`), `Sleep` beats sized to let output land
(longer after the card and tables). `Hide`/`Show` used only if an off-screen
step becomes unavoidable — prefer none.

## Testing Strategy

No Go code changes, so no unit tests. Verification is execution:

- `make-demo.sh` exits 0 and produces a non-empty `demo/out/demo.mp4` and `.gif`.
- Manual review of the rendered video: every command's real output visible,
  detail card renders as a box (VHS provides a pty, satisfying the TTY gate in
  `internal/cli/detail_card.go:92`), no error text, no truncated tables wider
  than the terminal.
- Run `make-demo.sh` twice; second run must also succeed (idempotent sync, new
  ids re-resolved).

## Boundaries

- **Always:** run against sandbox; read-only API usage plus `sync`; resolve ids
  at render time, never hardcode; rebuild the CLI before rendering.
- **Ask first:** any CLI source change beyond the demo (e.g. fixing the stray
  `0 results (live)` status line — see Open Questions); adding deps beyond vhs;
  creating any data via the API to make analytics richer.
- **Never:** touch production credentials/environment; commit rendered media or
  `demo/out/`; change agent/JSON output paths.

## Success Criteria

1. `./demo/make-demo.sh` runs unattended to completion on this machine.
2. Output video is 45–105s, shows all six beats, each with real output.
   (Rendered take is ~52s; lengthen the tape's `Sleep` beats to slow it down.)
3. Beat 3 renders the framed detail card (not JSON fallback).
4. No 404s, no stale-cache warnings, no visible error/warning text in the take.
5. Re-render after a CLI rebuild requires no manual editing.

## Resolved During Implementation

1. The stray `0 results` provenance line was a real bug: single-object gets
   counted the response by unmarshaling into an array, which fails for an
   object. Fixed (approved) by aligning the 26 generated call sites with the
   promoted commands' correct fallback (count a single object as 1).
2. A second glitch (`warning: N items skipped`) exists only on the live get
   path; the demo reads from the local mirror (hidden alias adds
   `--human-friendly --data-source local`), which also makes the
   `(cached, synced just now)` provenance line part of the story.
3. `returns` swapped for `pipeline` (see beat table). Theme: Catppuccin Mocha,
   colorful window bar, 1600x1000 @ FontSize 22.
4. Known cosmetic leftover: the `sql` beat's table header columns align
   slightly narrower than the data rows. Pre-existing `printAutoTable`
   behavior, not demo-specific.
