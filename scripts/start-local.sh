#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PORT="${PORT:-8080}"

cd "$ROOT_DIR"

if [ ! -f "autorun-go-pwa/dist/index.html" ]; then
  echo "Frontend build not found. Run scripts/package-local.sh first, or build autorun-go-pwa."
  exit 1
fi

export PORT
export FRONTEND_DIST="$ROOT_DIR/autorun-go-pwa/dist"

go run . &
SERVER_PID=$!

cleanup() {
  kill "$SERVER_PID" >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

echo "AutoRun is running at http://localhost:${PORT}"
echo "Press Ctrl+C to stop."
wait "$SERVER_PID"
