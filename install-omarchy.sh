#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
bash "$ROOT/install.sh"
command -v omarchy >/dev/null 2>&1 || { echo "Omarchy nicht gefunden; Standalone-Launcher ist trotzdem installiert."; exit 0; }
DEST="$HOME/.config/omarchy/plugins/local.omarchy-citizen"
mkdir -p "$DEST/backend/bin"
cp -f "$ROOT/integrations/omarchy/manifest.json" "$DEST/manifest.json"
cp -f "$ROOT/integrations/omarchy/BarWidget.qml" "$DEST/BarWidget.qml"
cp -f "$ROOT/integrations/omarchy/Panel.qml" "$DEST/Panel.qml"
cp -f "$ROOT/integrations/omarchy/citizenctl" "$DEST/citizenctl"
cp -f "$ROOT/integrations/omarchy/support-sanitize.py" "$DEST/support-sanitize.py"
cp -f "$ROOT/backend/bin/citizen-launcher" "$DEST/backend/bin/citizen-launcher"
chmod +x "$DEST/citizenctl" "$DEST/backend/bin/citizen-launcher"
omarchy plugin validate "$DEST"
omarchy-shell shell rescanPlugins >/dev/null 2>&1 || true
omarchy plugin enable local.omarchy-citizen >/dev/null 2>&1 || true
omarchy bar move local.omarchy-citizen --section right >/dev/null 2>&1 || true
omarchy restart shell >/dev/null 2>&1 || true
echo "Citizen Launcher + Omarchy Integration installiert."
