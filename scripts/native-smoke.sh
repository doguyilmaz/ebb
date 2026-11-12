#!/usr/bin/env bash
# Uses a fresh fake home. The installed ebb and real home are never used.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"
swift build --package-path macos --product EbbNativeSmoke
swift_bin="$(swift build --package-path macos --show-bin-path)"
mkdir -p bin/native-smoke
go build -o bin/native-smoke/ebb ./cmd/ebb
demo="$(mktemp -d)"
log="$(mktemp)"
pid=""
cleanup() {
  if [[ -n "$pid" ]]; then
    kill "$pid" 2>/dev/null || true
    wait "$pid" 2>/dev/null || true
  fi
  rm -rf "$demo" "$log"
}
trap cleanup EXIT
scripts/demo-home.sh "$demo" >/dev/null
port="${NATIVE_SMOKE_PORT:-47432}"
EBB_HOME="$demo" EBB_SANDBOX=1 bin/native-smoke/ebb serve --port "$port" > "$log" 2>&1 &
pid=$!
for _ in $(seq 1 100); do
  [[ -f "$demo/.local/state/ebb/server.json" ]] && break
  kill -0 "$pid" 2>/dev/null || { echo "sandbox engine did not start" >&2; exit 1; }
  sleep 0.1
done
EBB_HOME="$demo" EBB_SANDBOX=1 EBB_PORT="$port" "$swift_bin/EbbNativeSmoke"
