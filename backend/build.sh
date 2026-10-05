#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$(dirname "$0")"
VERSION="$(tr -d '[:space:]' < "$ROOT/VERSION")"
RELEASE_REPO="${CITIZEN_LAUNCHER_RELEASE_REPO:-github:b1tk1ll3r/sc-lnx-inst}"
mkdir -p bin
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -trimpath -buildvcs=false -ldflags="-s -w -B gobuildid -X main.appVersion=$VERSION -X main.releaseRepo=$RELEASE_REPO" \
  -o bin/citizen-launcher \
  ./cmd/citizen-launcher
sha256sum bin/citizen-launcher
