#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
"$ROOT/tests/verify.sh"
(cd "$ROOT/backend" && go test -race ./...)
"$ROOT/backend/integration-test.sh"
"$ROOT/tests/package-verify.sh"
echo 'Citizen Launcher full verification: OK'
