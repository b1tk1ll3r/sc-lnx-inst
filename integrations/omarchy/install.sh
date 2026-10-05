#!/usr/bin/env bash
# Optional Omarchy bar widget for Citizen Launcher (run from a source checkout).
# Requires an installed citizen-launcher (see ../../install.sh); the widget
# calls the installed binary, so it is updated together with the launcher.
set -euo pipefail
HERE="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
command -v citizen-launcher >/dev/null 2>&1 || { echo "citizen-launcher ist nicht installiert. Zuerst install.sh ausführen." >&2; exit 1; }
command -v omarchy >/dev/null 2>&1 || { echo "Omarchy nicht gefunden." >&2; exit 1; }
DEST="$HOME/.config/omarchy/plugins/local.omarchy-citizen"
mkdir -p "$DEST"
for f in manifest.json BarWidget.qml Panel.qml citizenctl support-sanitize.py; do
  install -m644 "$HERE/$f" "$DEST/$f"
done
chmod 755 "$DEST/citizenctl"
omarchy plugin validate "$DEST"
omarchy-shell shell rescanPlugins >/dev/null 2>&1 || true
omarchy plugin enable local.omarchy-citizen >/dev/null 2>&1 || true
omarchy bar move local.omarchy-citizen --section right >/dev/null 2>&1 || true
omarchy restart shell >/dev/null 2>&1 || true
echo "Omarchy-Widget für Citizen Launcher installiert."
