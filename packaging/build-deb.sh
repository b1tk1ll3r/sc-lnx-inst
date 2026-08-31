#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="$(tr -d '[:space:]' < "$ROOT/VERSION")"
ARCH="amd64"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
PKG="$WORK/citizen-launcher_${VERSION}_${ARCH}"
mkdir -p \
  "$PKG/DEBIAN" \
  "$PKG/usr/bin" \
  "$PKG/usr/share/applications" \
  "$PKG/usr/share/icons/hicolor/scalable/apps" \
  "$PKG/usr/share/metainfo" \
  "$PKG/usr/lib/systemd/system" \
  "$PKG/usr/lib/sysctl.d" \
  "$PKG/etc/security/limits.d" \
  "$PKG/etc/xdg/autostart" \
  "$PKG/etc/citizen-launcher"

install -m755 "$ROOT/backend/bin/citizen-launcher" "$PKG/usr/bin/citizen-launcher"
install -m644 "$ROOT/packaging/systemd/citizen-launcher-self-update.service" "$PKG/usr/lib/systemd/system/citizen-launcher-self-update.service"
install -m644 "$ROOT/packaging/systemd/citizen-launcher-self-update.timer" "$PKG/usr/lib/systemd/system/citizen-launcher-self-update.timer"
install -m644 "$ROOT/packaging/icons/citizen-launcher.svg" "$PKG/usr/share/icons/hicolor/scalable/apps/citizen-launcher.svg"
install -m644 "$ROOT/packaging/metainfo/io.github.citizenlauncher.CitizenLauncher.metainfo.xml" "$PKG/usr/share/metainfo/io.github.citizenlauncher.CitizenLauncher.metainfo.xml"
install -m644 "$ROOT/packaging/citizen-launcher-migrate.desktop" "$PKG/etc/xdg/autostart/citizen-launcher-migrate.desktop"
printf '%s\n' "${CITIZEN_LAUNCHER_RELEASE_REPO:-sendnwv/omarchy-sc}" > "$PKG/etc/citizen-launcher/release-repo"
printf '%s\n' '# Citizen Launcher / Star Citizen' 'vm.max_map_count = 16777216' > "$PKG/usr/lib/sysctl.d/90-citizen-launcher.conf"
printf '%s\n' '# Citizen Launcher / Star Citizen' '* soft nofile 524288' '* hard nofile 524288' > "$PKG/etc/security/limits.d/90-citizen-launcher.conf"
install -m755 "$ROOT/packaging/postinst" "$PKG/DEBIAN/postinst"
install -m755 "$ROOT/packaging/prerm" "$PKG/DEBIAN/prerm"
install -m755 "$ROOT/packaging/postrm" "$PKG/DEBIAN/postrm"

cat > "$PKG/DEBIAN/control" <<EOF2
Package: citizen-launcher
Version: $VERSION
Section: games
Priority: optional
Architecture: $ARCH
Maintainer: Citizen Launcher Project
Depends: ca-certificates, tar, xdg-utils, curl, cabextract, unzip, xz-utils, apt, procps, util-linux, pkexec
Recommends: pciutils, vulkan-tools
Description: Star Citizen setup, launcher and self-maintaining gaming stack for Linux
 Citizen Launcher manages Wine, DXVK, the RSI Launcher, automatic maintenance,
 desktop integration and privacy-conscious support diagnostics.
EOF2

cat > "$PKG/usr/share/applications/io.github.citizenlauncher.CitizenLauncher.desktop" <<'EOF2'
[Desktop Entry]
Name=Citizen Launcher
Comment=Star Citizen for Linux
Exec=/usr/bin/citizen-launcher gui
TryExec=/usr/bin/citizen-launcher
Icon=citizen-launcher
Terminal=false
Type=Application
Categories=Game;
StartupNotify=true
StartupWMClass=CitizenLauncher
Keywords=Star Citizen;RSI;Wine;Gaming;
EOF2

mkdir -p "$ROOT/dist"
dpkg-deb --build --root-owner-group "$PKG" "$ROOT/dist/citizen-launcher_${VERSION}_${ARCH}.deb"
