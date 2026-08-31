#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

for f in citizenctl install.sh INSTALLIEREN.sh uninstall.sh; do
  bash -n "$f"
done
python3 -m py_compile support-sanitize.py

(
  cd backend
  gofmt -w cmd/omarchy-citizen-backend/*.go
  go test ./...
  go vet ./...
  bash build.sh
)

bash tests/smoke-citizenctl.sh

echo "Omarchy Citizen verification: OK"
