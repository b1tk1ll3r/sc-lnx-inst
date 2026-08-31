#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="$(tr -d '[:space:]' < "$ROOT/VERSION")"; ARCH="amd64"; WORK="$(mktemp -d)"; trap 'rm -rf "$WORK"' EXIT
PKG="$WORK/citizen-launcher_${VERSION}_${ARCH}"
mkdir -p "$PKG/DEBIAN" "$PKG/usr/bin" "$PKG/usr/share/applications" "$PKG/usr/lib/systemd/system" "$PKG/etc/citizen-launcher"
install -m755 "$ROOT/backend/bin/citizen-launcher" "$PKG/usr/bin/citizen-launcher"

install -m644 "$ROOT/packaging/systemd/citizen-launcher-self-update.service" "$PKG/usr/lib/systemd/system/citizen-launcher-self-update.service"
install -m644 "$ROOT/packaging/systemd/citizen-launcher-self-update.timer" "$PKG/usr/lib/systemd/system/citizen-launcher-self-update.timer"
printf '%s\n' "${CITIZEN_LAUNCHER_RELEASE_REPO:-sendnwv/omarchy-sc}" > "$PKG/etc/citizen-launcher/release-repo"
install -m755 "$ROOT/packaging/postinst" "$PKG/DEBIAN/postinst"
install -m755 "$ROOT/packaging/prerm" "$PKG/DEBIAN/prerm"
install -m755 "$ROOT/packaging/postrm" "$PKG/DEBIAN/postrm"
cat > "$PKG/DEBIAN/control" <<EOF
Package: citizen-launcher
Version: $VERSION
Section: games
Priority: optional
Architecture: $ARCH
Maintainer: Citizen Launcher Project
Depends: ca-certificates, tar, xdg-utils, curl, cabextract, unzip, xz-utils, apt
Recommends: pciutils, vulkan-tools
Description: Star Citizen setup, launcher and self-maintaining gaming stack for Linux
 Distro-neutral Citizen Launcher with Wine/DXVK/RSI management and local GUI.
EOF
cat > "$PKG/usr/share/applications/io.github.citizenlauncher.CitizenLauncher.desktop" <<EOF
[Desktop Entry]
Name=Citizen Launcher
Comment=Star Citizen for Linux
Exec=citizen-launcher gui
Terminal=false
Type=Application
Categories=Game;
StartupNotify=true
EOF
mkdir -p "$ROOT/dist"
dpkg-deb --build --root-owner-group "$PKG" "$ROOT/dist/citizen-launcher_${VERSION}_${ARCH}.deb"
