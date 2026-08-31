#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
for f in build.sh install.sh install-omarchy.sh uninstall.sh packaging/build-deb.sh packaging/build-tarball.sh backend/integration-test.sh integrations/omarchy/citizenctl; do
  bash -n "$ROOT/$f"
done
UNFORMATTED="$(cd "$ROOT/backend" && gofmt -l cmd/citizen-launcher/*.go)"
if [[ -n "$UNFORMATTED" ]]; then
  echo "Go source is not gofmt-clean:" >&2
  echo "$UNFORMATTED" >&2
  exit 1
fi
(cd "$ROOT/backend" && go test ./... && go vet ./... && ./build.sh)
"$ROOT/backend/bin/citizen-launcher" --version | grep -qx "$(tr -d '[:space:]' < "$ROOT/VERSION")"
"$ROOT/backend/bin/citizen-launcher" platform --json | grep -q '"id"'
grep -a -q 'FLIGHT READY' "$ROOT/backend/bin/citizen-launcher"
echo 'Citizen Launcher verification: OK'
