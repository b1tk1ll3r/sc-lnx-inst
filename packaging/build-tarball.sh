#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
mkdir -p "$ROOT/dist"
VERSION="$(tr -d '[:space:]' < "$ROOT/VERSION")"
tar -C "$ROOT/backend/bin" -czf "$ROOT/dist/citizen-launcher-${VERSION}-linux-amd64.tar.gz" citizen-launcher
