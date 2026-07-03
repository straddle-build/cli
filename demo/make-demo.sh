#!/usr/bin/env bash
# Render the marketing demo: build the CLI, freshen sandbox data, resolve a
# current customer id, generate the tape from the template, and run vhs.
# Output: demo/out/demo.mp4 + demo/out/demo.gif
set -euo pipefail
cd "$(dirname "$0")/.."

command -v vhs >/dev/null || { echo "vhs not installed (brew install vhs)" >&2; exit 1; }

go build -o ./straddle-pp-cli ./cmd/straddle-pp-cli

# Off-camera full sync so every resource is fresh: doctor shows a clean cache
# and the on-camera scoped sync is a fast re-pull, never a cold crawl.
./straddle-pp-cli sync --full >/dev/null

CUSTOMER_ID=$(./straddle-pp-cli sql \
  "select id from customers order by created_at desc limit 1" --csv | tail -1)
[ -n "$CUSTOMER_ID" ] || { echo "no customers in local store after sync" >&2; exit 1; }

mkdir -p demo/out
sed -e "s|{{CUSTOMER_ID}}|$CUSTOMER_ID|g" \
    -e "s|{{REPO_DIR}}|$PWD|g" \
    demo/demo.tape.tmpl > demo/out/demo.tape

vhs demo/out/demo.tape

ls -lh demo/out/demo.gif demo/out/demo.mp4
