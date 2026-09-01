#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="$(tr -d '[:space:]' < "$ROOT/VERSION")"
rm -rf "$ROOT/dist"
mkdir -p "$ROOT/dist"
bash "$ROOT/packaging/build-deb.sh"
bash "$ROOT/packaging/build-tarball.sh"
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
  etc/citizen-launcher/release-repo \
  usr/share/applications/io.github.citizenlauncher.CitizenLauncher.desktop \
  usr/share/metainfo/io.github.citizenlauncher.CitizenLauncher.metainfo.xml; do
  [[ -f "$TMP/root/$f" ]] || { echo "Missing DEB payload: $f" >&2; exit 1; }
done
mkdir -p "$TMP/tar"
tar -xzf "$TAR" -C "$TMP/tar"
[[ "$("$TMP/tar/citizen-launcher" --version)" == "$VERSION" ]]
grep -q '^Exec=/usr/bin/citizen-launcher gui$' "$TMP/root/usr/share/applications/io.github.citizenlauncher.CitizenLauncher.desktop"
grep -q '^ExecStart=/usr/bin/citizen-launcher self-update apply --system --quiet$' "$TMP/root/usr/lib/systemd/system/citizen-launcher-self-update.service"
grep -q 'zstd' <(dpkg-deb -f "$DEB" Depends)

# Packaging definitions for the other native families are validated on every
# host; their actual package builders are executed when their native tools exist
# and in GitHub's Fedora/Arch CI jobs.
bash -n "$ROOT/INSTALLIEREN.sh" "$ROOT/packaging/build-rpm.sh" "$ROOT/packaging/build-arch.sh" "$ROOT/packaging/build-all.sh"
grep -q '^Name:[[:space:]]*citizen-launcher$' "$ROOT/packaging/rpm/citizen-launcher.spec"
grep -q 'citizen-launcher-self-update.timer' "$ROOT/packaging/rpm/citizen-launcher.spec"
grep -q '^pkgname=citizen-launcher$' "$ROOT/packaging/arch/PKGBUILD.in"
grep -q "'x86_64'" "$ROOT/packaging/arch/PKGBUILD.in"
grep -q 'citizen-launcher-self-update.timer' "$ROOT/packaging/arch/PKGBUILD.in"

if command -v rpmbuild >/dev/null 2>&1; then
  bash "$ROOT/packaging/build-rpm.sh"
  RPM="$ROOT/dist/citizen-launcher-${VERSION}-1.linux.x86_64.rpm"
  [[ -s "$RPM" ]]
  rpm -qp --qf '%{NAME}\n%{VERSION}\n%{ARCH}\n' "$RPM" | grep -qx 'citizen-launcher' -m1
fi

if command -v makepkg >/dev/null 2>&1 && [[ $EUID -ne 0 ]]; then
  bash "$ROOT/packaging/build-arch.sh"
  [[ -s "$ROOT/dist/citizen-launcher-${VERSION}-1-x86_64.pkg.tar.zst" ]]
fi

echo 'Citizen Launcher package verification: OK'
