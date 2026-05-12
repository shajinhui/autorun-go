#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

GOOS=linux GOARCH=amd64 go build -o main ./cmd/scf
zip -j main.zip main

echo "SCF build complete: $(pwd)/main.zip"
