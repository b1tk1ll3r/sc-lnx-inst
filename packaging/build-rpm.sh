#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="$(tr -d '[:space:]' < "$ROOT/VERSION")"
command -v rpmbuild >/dev/null 2>&1 || { echo "rpmbuild fehlt. Unter Fedora: sudo dnf install rpm-build" >&2; exit 2; }
[[ -x "$ROOT/backend/bin/citizen-launcher" ]] || { echo "Backend fehlt: zuerst ./build.sh" >&2; exit 1; }
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
TOP="$WORK/rpmbuild"
mkdir -p "$TOP"/{BUILD,BUILDROOT,RPMS,SOURCES,SPECS,SRPMS}
install -m755 "$ROOT/backend/bin/citizen-launcher" "$TOP/SOURCES/citizen-launcher"
install -m644 "$ROOT/packaging/common/io.github.citizenlauncher.CitizenLauncher.desktop" "$TOP/SOURCES/io.github.citizenlauncher.CitizenLauncher.desktop"
install -m644 "$ROOT/packaging/icons/citizen-launcher.svg" "$TOP/SOURCES/citizen-launcher.svg"
install -m644 "$ROOT/packaging/metainfo/io.github.citizenlauncher.CitizenLauncher.metainfo.xml" "$TOP/SOURCES/io.github.citizenlauncher.CitizenLauncher.metainfo.xml"
install -m644 "$ROOT/packaging/systemd/citizen-launcher-self-update.service" "$TOP/SOURCES/citizen-launcher-self-update.service"
install -m644 "$ROOT/packaging/systemd/citizen-launcher-self-update.timer" "$TOP/SOURCES/citizen-launcher-self-update.timer"
install -m644 "$ROOT/packaging/citizen-launcher-migrate.desktop" "$TOP/SOURCES/citizen-launcher-migrate.desktop"
install -m644 "$ROOT/packaging/common/90-citizen-launcher.conf" "$TOP/SOURCES/90-citizen-launcher.conf"
install -m644 "$ROOT/packaging/common/90-citizen-launcher-limits.conf" "$TOP/SOURCES/90-citizen-launcher-limits.conf"
if [[ -n "${CITIZEN_LAUNCHER_RELEASE_REPO:-}" ]]; then
  printf '%s\n' "$CITIZEN_LAUNCHER_RELEASE_REPO" > "$TOP/SOURCES/release-repo"
else
  install -m644 "$ROOT/packaging/common/release-repo" "$TOP/SOURCES/release-repo"
fi
install -m644 "$ROOT/LICENSE" "$TOP/SOURCES/LICENSE"
install -m644 "$ROOT/packaging/rpm/citizen-launcher.spec" "$TOP/SPECS/citizen-launcher.spec"
rpmbuild -bb \
  --define "_topdir $TOP" \
  --define "cl_version $VERSION" \
  --define "_unitdir /usr/lib/systemd/system" \
  "$TOP/SPECS/citizen-launcher.spec"
mkdir -p "$ROOT/dist"
RPM="$(find "$TOP/RPMS" -type f -name 'citizen-launcher-*.x86_64.rpm' -print -quit)"
[[ -n "$RPM" ]] || { echo "RPM wurde nicht erzeugt" >&2; exit 1; }
# Normalize the release asset name across Fedora/openSUSE so the secure updater
# has one deterministic RPM target.
cp -f "$RPM" "$ROOT/dist/citizen-launcher-${VERSION}-1.linux.x86_64.rpm"
echo "$ROOT/dist/citizen-launcher-${VERSION}-1.linux.x86_64.rpm"
