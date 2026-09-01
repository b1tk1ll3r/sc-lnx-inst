#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
bash "$ROOT/tests/verify.sh"
bash "$ROOT/tests/mode-independence.sh"
(cd "$ROOT/backend" && go test -race ./...)
bash "$ROOT/backend/integration-test.sh"
bash "$ROOT/tests/gitea-release-helper.sh"
bash "$ROOT/tests/package-verify.sh"
echo 'Citizen Launcher full verification: OK'
