#!/usr/bin/env bash
set -euo pipefail

PLUGIN_ID="local.omarchy-citizen"
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
DEST="$HOME/.config/omarchy/plugins/$PLUGIN_ID"

message() {
  local title="$1"
  local body="$2"
  if command -v zenity >/dev/null 2>&1; then
    zenity --info --width=500 --title="$title" --text="$body" >/dev/null 2>&1 || true
  elif command -v omarchy-notification-send >/dev/null 2>&1; then
    omarchy-notification-send "$title" "$body" >/dev/null 2>&1 || true
  elif command -v notify-send >/dev/null 2>&1; then
    notify-send "$title" "$body" >/dev/null 2>&1 || true
  fi
}

if ! command -v omarchy >/dev/null 2>&1; then
  message "Omarchy Citizen" "Omarchy wurde nicht gefunden. Dieses Plugin benötigt Omarchy Quattro."
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

chmod +x "$DEST/citizenctl" "$DEST/support-sanitize.py" "$DEST/install.sh" "$DEST/uninstall.sh" "$DEST/INSTALLIEREN.sh" "$DEST/backend/bin/omarchy-citizen-backend"

if ! omarchy plugin validate "$DEST" >/tmp/omarchy-citizen-plugin-validation.log 2>&1; then
  message "Omarchy Citizen" "Das Plugin konnte nicht validiert werden. Details stehen in /tmp/omarchy-citizen-plugin-validation.log"
  exit 1
fi

omarchy-shell shell rescanPlugins >/dev/null 2>&1 || true
"$DEST/backend/bin/omarchy-citizen-backend" self-sync >/dev/null 2>&1 || true
"$HOME/.local/lib/omarchy-citizen/omarchy-citizen-backend" install-service >/dev/null 2>&1 || true

omarchy plugin enable "$PLUGIN_ID" >/dev/null 2>&1 || true
omarchy bar move "$PLUGIN_ID" --section right >/dev/null 2>&1 || true

# QML hot-reload is currently unreliable for third-party bar plugins.
omarchy restart shell >/dev/null 2>&1 || true

message "Omarchy Citizen" "Installation abgeschlossen.

Oben in der Omarchy-Leiste findest du jetzt ✦SC.

Klicke darauf und anschließend auf „EINRICHTEN & STARTKLAR MACHEN“."
