#!/usr/bin/env bash
# Clicks through the interface in a real browser against a fresh sandboxed demo
# home and checks where every action lands: in the UI, through the API and on
# disk. Nothing outside the demo home is touched.
#   scripts/e2e.sh                 (E2E_SHOTS=dir also saves screenshots)
# Uses the installed Google Chrome; set CHROMIUM to use another one.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
export EBB_E2E_BIN="$ROOT/bin/ebb-e2e"
[ "${SKIP_UI_BUILD:-}" = 1 ] || (cd ui && bun run build >/dev/null)
go build -o "$EBB_E2E_BIN" ./cmd/ebb 2>/dev/null

DEMO="$(mktemp -d)"
scripts/demo-home.sh "$DEMO" >/dev/null
export EBB_E2E_HOME="$DEMO"
PORT="${E2E_PORT:-47411}"
LOG="$(mktemp)"
EBB_HOME="$DEMO" EBB_SANDBOX=1 "$EBB_E2E_BIN" serve --port "$PORT" >"$LOG" 2>&1 &
pid=$!
trap 'kill $pid 2>/dev/null || true; rm -rf "$LOG" "$DEMO"' EXIT
for _ in $(seq 1 50); do grep -q token= "$LOG" && break; sleep 0.1; done
export EBB_E2E_TOKEN="$(grep -o 'token=[0-9a-f]*' "$LOG" | cut -d= -f2)"
export EBB_E2E_PORT="$PORT"
node ui/e2e/audit.mjs
