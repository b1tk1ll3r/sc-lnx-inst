#!/usr/bin/env bash
set -euo pipefail

PLUGIN_ID="local.omarchy-citizen"
DEST="$HOME/.config/omarchy/plugins/$PLUGIN_ID"

BACKEND="$HOME/.local/lib/omarchy-citizen/omarchy-citizen-backend"
[[ -x "$BACKEND" ]] && "$BACKEND" uninstall-service >/dev/null 2>&1 || true
omarchy plugin disable "$PLUGIN_ID" 2>/dev/null || true

BACKEND="$HOME/.local/lib/omarchy-citizen/omarchy-citizen-backend"
if [[ -x "$BACKEND" ]]; then
  "$BACKEND" auto disable >/dev/null 2>&1 || true
  "$BACKEND" uninstall-service >/dev/null 2>&1 || true
fi
rm -f -- "$BACKEND"
rmdir "$HOME/.local/lib/omarchy-citizen" 2>/dev/null || true

if [[ -d "$DEST" ]]; then
  rm -rf -- "$DEST"
fi

omarchy-shell shell rescanPlugins 2>/dev/null || true
echo "Omarchy Citizen wurde entfernt."
echo "Star Citizen und der LUG Helper wurden NICHT entfernt."
