#!/usr/bin/env bash
# Quick smoke of the create surface: dry-run a sandbox charge and show the
# request without sending it. Ported from the v1 CLI's demo-charge.sh
# (straddle-cli --human -> straddle-pp-cli --human-friendly).
set -euo pipefail
cd "$(dirname "$0")/.."

[ -x ./straddle-pp-cli ] || go build -o ./straddle-pp-cli ./cmd/straddle-pp-cli

./straddle-pp-cli charges create \
  --paykey pk_demo \
  --amount 1234 \
  --currency USD \
  --payment-date "$(date +%F)" \
  --consent-type internet \
  --description demo \
  --dry-run --human-friendly
