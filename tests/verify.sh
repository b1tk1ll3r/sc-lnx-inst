#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
for f in build.sh install.sh install-omarchy.sh uninstall.sh packaging/build-deb.sh packaging/build-tarball.sh integrations/omarchy/citizenctl; do bash -n "$ROOT/$f"; done
(cd "$ROOT/backend" && gofmt -w cmd/citizen-launcher/*.go && go test ./... && go vet ./... && ./build.sh)
"$ROOT/backend/bin/citizen-launcher" --version | grep -qx '0.9.0'
"$ROOT/backend/bin/citizen-launcher" platform --json | grep -q '"id"'
# Ensure GUI assets are embedded by looking for a distinctive string in binary.
grep -a -q 'FLIGHT READY' "$ROOT/backend/bin/citizen-launcher"
echo 'Citizen Launcher verification: OK'
