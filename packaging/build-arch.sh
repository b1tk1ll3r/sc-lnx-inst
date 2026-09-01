#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="$(tr -d '[:space:]' < "$ROOT/VERSION")"
command -v makepkg >/dev/null 2>&1 || { echo "makepkg fehlt. Unter Arch: sudo pacman -S --needed base-devel" >&2; exit 2; }
[[ "$(id -u)" -ne 0 ]] || { echo "makepkg darf nicht als root laufen" >&2; exit 2; }
[[ -x "$ROOT/backend/bin/citizen-launcher" ]] || { echo "Backend fehlt: zuerst ./build.sh" >&2; exit 1; }
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
sed "s/@VERSION@/$VERSION/g" "$ROOT/packaging/arch/PKGBUILD.in" > "$WORK/PKGBUILD"
install -m755 "$ROOT/backend/bin/citizen-launcher" "$WORK/citizen-launcher"
install -m644 "$ROOT/packaging/common/io.github.citizenlauncher.CitizenLauncher.desktop" "$WORK/io.github.citizenlauncher.CitizenLauncher.desktop"
install -m644 "$ROOT/packaging/icons/citizen-launcher.svg" "$WORK/citizen-launcher.svg"
install -m644 "$ROOT/packaging/metainfo/io.github.citizenlauncher.CitizenLauncher.metainfo.xml" "$WORK/io.github.citizenlauncher.CitizenLauncher.metainfo.xml"
install -m644 "$ROOT/packaging/systemd/citizen-launcher-self-update.service" "$WORK/citizen-launcher-self-update.service"
install -m644 "$ROOT/packaging/systemd/citizen-launcher-self-update.timer" "$WORK/citizen-launcher-self-update.timer"
install -m644 "$ROOT/packaging/citizen-launcher-migrate.desktop" "$WORK/citizen-launcher-migrate.desktop"
install -m644 "$ROOT/packaging/common/90-citizen-launcher.conf" "$WORK/90-citizen-launcher.conf"
install -m644 "$ROOT/packaging/common/90-citizen-launcher-limits.conf" "$WORK/90-citizen-launcher-limits.conf"
if [[ -n "${CITIZEN_LAUNCHER_RELEASE_REPO:-}" ]]; then
  printf '%s\n' "$CITIZEN_LAUNCHER_RELEASE_REPO" > "$WORK/release-repo"
else
  install -m644 "$ROOT/packaging/common/release-repo" "$WORK/release-repo"
fi
install -m644 "$ROOT/LICENSE" "$WORK/LICENSE"
install -m644 "$ROOT/packaging/common/citizen-launcher.package-install" "$WORK/citizen-launcher.install"
(
  cd "$WORK"
  PKGDEST="$ROOT/dist" makepkg --cleanbuild --clean --force --nodeps --noconfirm
)
PKG="$ROOT/dist/citizen-launcher-${VERSION}-1-x86_64.pkg.tar.zst"
[[ -s "$PKG" ]] || { echo "Arch-Paket wurde nicht erzeugt: $PKG" >&2; exit 1; }
echo "$PKG"
