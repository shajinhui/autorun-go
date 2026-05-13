#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PORT="${PORT:-8080}"

cd "$ROOT_DIR"
export PORT

go run . &
SERVER_PID=$!

cleanup() {
  kill "$SERVER_PID" >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

cd "$ROOT_DIR/autorun-go-pwa"
if command -v pnpm >/dev/null 2>&1; then
  pnpm run dev --host 0.0.0.0
else
  npm run dev -- --host 0.0.0.0
fi
