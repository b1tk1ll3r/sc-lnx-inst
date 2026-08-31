#!/usr/bin/env bash
set -euo pipefail

PLUGIN_ID="local.omarchy-citizen"
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
DEST="$HOME/.config/omarchy/plugins/$PLUGIN_ID"

if ! command -v omarchy >/dev/null 2>&1; then
  echo "Fehler: 'omarchy' wurde nicht gefunden."
  echo "Dieses Plugin benötigt Omarchy Quattro mit dem Plugin-System."
  exit 1
fi

mkdir -p "$DEST"

for file in manifest.json BarWidget.qml Panel.qml citizenctl support-sanitize.py README.md INSTALLATION.md LICENSE uninstall.sh install.sh INSTALLIEREN.sh; do
  src="$SCRIPT_DIR/$file"
  dst="$DEST/$file"
  if [[ "$(readlink -f "$src")" != "$(readlink -f "$dst" 2>/dev/null || printf '%s' "$dst")" ]]; then
    cp -f -- "$src" "$dst"
  fi
done

if [[ "$(readlink -f "$SCRIPT_DIR/backend")" != "$(readlink -f "$DEST/backend" 2>/dev/null || printf '%s' "$DEST/backend")" ]]; then
  rm -rf -- "$DEST/backend"
  cp -a -- "$SCRIPT_DIR/backend" "$DEST/backend"
fi

chmod +x "$DEST/citizenctl" "$DEST/support-sanitize.py" "$DEST/uninstall.sh" "$DEST/INSTALLIEREN.sh" "$DEST/backend/bin/omarchy-citizen-backend"

echo "Validiere Plugin..."
omarchy plugin validate "$DEST"

echo "Lade Plugin neu..."
omarchy-shell shell rescanPlugins

echo "Aktiviere Plugin..."
"$DEST/backend/bin/omarchy-citizen-backend" self-sync >/dev/null 2>&1 || true
"$HOME/.local/lib/omarchy-citizen/omarchy-citizen-backend" install-service >/dev/null 2>&1 || true

omarchy plugin enable "$PLUGIN_ID"

# QML hot-reload is currently unreliable for third-party bar plugins.
omarchy restart shell >/dev/null 2>&1 || true

echo
echo "Omarchy Citizen 0.6.1 wurde installiert:"
echo "  $DEST"
echo
echo "Links-Klick auf 'SC' in der Omarchy-Leiste öffnet das Star-Citizen-Panel."
echo
echo "Falls 'SC' nicht sichtbar ist:"
echo "  omarchy bar move $PLUGIN_ID --section right"
echo
echo "LUG Helper Status:"
bash "$DEST/citizenctl" status
