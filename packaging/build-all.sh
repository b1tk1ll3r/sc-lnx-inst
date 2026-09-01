#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
bash "$ROOT/build.sh"
bash "$ROOT/packaging/build-deb.sh"
bash "$ROOT/packaging/build-tarball.sh"
if command -v rpmbuild >/dev/null 2>&1; then
  bash "$ROOT/packaging/build-rpm.sh"
else
  echo "Hinweis: rpmbuild fehlt – RPM wird in Fedora/openSUSE CI gebaut." >&2
fi
if command -v makepkg >/dev/null 2>&1 && [[ $EUID -ne 0 ]]; then
  bash "$ROOT/packaging/build-arch.sh"
else
  echo "Hinweis: makepkg fehlt/Root – Arch-Paket wird in Arch CI gebaut." >&2
fi
