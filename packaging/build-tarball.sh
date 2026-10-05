#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
mkdir -p "$ROOT/dist"
VERSION="$(tr -d '[:space:]' < "$ROOT/VERSION")"
# Binary plus icon for the user-mode installer; the self-updater only extracts the binary.
STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT
install -m755 "$ROOT/backend/bin/citizen-launcher" "$STAGE/citizen-launcher"
install -m644 "$ROOT/packaging/icons/citizen-launcher.svg" "$STAGE/citizen-launcher.svg"
tar -C "$STAGE" --owner=0 --group=0 --numeric-owner -czf "$ROOT/dist/citizen-launcher-${VERSION}-linux-amd64.tar.gz" citizen-launcher citizen-launcher.svg
