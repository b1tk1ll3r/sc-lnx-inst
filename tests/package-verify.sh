#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="$(tr -d '[:space:]' < "$ROOT/VERSION")"
rm -rf "$ROOT/dist"
mkdir -p "$ROOT/dist"
"$ROOT/packaging/build-deb.sh"
"$ROOT/packaging/build-tarball.sh"
DEB="$ROOT/dist/citizen-launcher_${VERSION}_amd64.deb"
TAR="$ROOT/dist/citizen-launcher-${VERSION}-linux-amd64.tar.gz"
[[ -s "$DEB" && -s "$TAR" ]]
[[ "$(dpkg-deb -f "$DEB" Package)" == "citizen-launcher" ]]
[[ "$(dpkg-deb -f "$DEB" Version)" == "$VERSION" ]]
[[ "$(dpkg-deb -f "$DEB" Architecture)" == "amd64" ]]
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
dpkg-deb -x "$DEB" "$TMP/root"
[[ "$("$TMP/root/usr/bin/citizen-launcher" --version)" == "$VERSION" ]]
for f in \
  usr/lib/systemd/system/citizen-launcher-self-update.service \
  usr/lib/systemd/system/citizen-launcher-self-update.timer \
  usr/lib/sysctl.d/90-citizen-launcher.conf \
  etc/security/limits.d/90-citizen-launcher.conf \
  etc/xdg/autostart/citizen-launcher-migrate.desktop \
  usr/share/applications/io.github.citizenlauncher.CitizenLauncher.desktop \
  usr/share/metainfo/io.github.citizenlauncher.CitizenLauncher.metainfo.xml; do
  [[ -f "$TMP/root/$f" ]] || { echo "Missing DEB payload: $f" >&2; exit 1; }
done
mkdir -p "$TMP/tar"
tar -xzf "$TAR" -C "$TMP/tar"
[[ "$("$TMP/tar/citizen-launcher" --version)" == "$VERSION" ]]
grep -q '^Exec=/usr/bin/citizen-launcher gui$' "$TMP/root/usr/share/applications/io.github.citizenlauncher.CitizenLauncher.desktop"
grep -q '^ExecStart=/usr/bin/citizen-launcher self-update apply --system --quiet$' "$TMP/root/usr/lib/systemd/system/citizen-launcher-self-update.service"
echo 'Citizen Launcher package verification: OK'
