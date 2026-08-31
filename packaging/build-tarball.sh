#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
mkdir -p "$ROOT/dist"
tar -C "$ROOT/backend/bin" -czf "$ROOT/dist/citizen-launcher-0.9.1-linux-amd64.tar.gz" citizen-launcher
