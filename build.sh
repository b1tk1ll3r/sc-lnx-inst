#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
(cd "$ROOT/backend" && ./build.sh)
echo "Build fertig: backend/bin/citizen-launcher"
