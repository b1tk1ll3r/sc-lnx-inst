#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
"$ROOT/build.sh"
"$ROOT/packaging/build-deb.sh"
"$ROOT/packaging/build-tarball.sh"
if command -v rpmbuild >/dev/null 2>&1; then
  "$ROOT/packaging/build-rpm.sh"
else
  echo "Hinweis: rpmbuild fehlt – RPM wird in Fedora/openSUSE CI gebaut." >&2
fi
if command -v makepkg >/dev/null 2>&1 && [[ $EUID -ne 0 ]]; then
  "$ROOT/packaging/build-arch.sh"
else
  echo "Hinweis: makepkg fehlt/Root – Arch-Paket wird in Arch CI gebaut." >&2
fi
