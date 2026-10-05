#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
for f in \
  "$ROOT/install.sh" "$ROOT/integrations/omarchy/install.sh" "$ROOT/integrations/omarchy/citizenctl" \
  "$ROOT/tests/installer-test.sh" "$ROOT/tests/smoke-citizenctl.sh" \
  "$ROOT/build.sh" "$ROOT/packaging/build-deb.sh" "$ROOT/packaging/build-rpm.sh" \
  "$ROOT/packaging/build-arch.sh" "$ROOT/packaging/build-tarball.sh" "$ROOT/packaging/build-all.sh" \
  "$ROOT/tests/mode-independence.sh" "$ROOT/tests/workflow-policy.sh"; do
  bash -n "$f"
done
# Fail on unformatted code instead of silently rewriting it.
unformatted="$(gofmt -l "$ROOT/backend")"
[[ -z "$unformatted" ]] || { echo "gofmt: needs formatting:" >&2; echo "$unformatted" >&2; exit 1; }
(cd "$ROOT/backend" && go test ./... && go vet ./...)
bash "$ROOT/build.sh"
[[ "$("$ROOT/backend/bin/citizen-launcher" --version)" == "$(tr -d '[:space:]' < "$ROOT/VERSION")" ]]
echo 'Citizen Launcher verification: OK'
