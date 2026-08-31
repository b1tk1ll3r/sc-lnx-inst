#!/usr/bin/env bash
set -euo pipefail
BIN="$HOME/.local/bin/citizen-launcher"
[[ -x "$BIN" ]] && "$BIN" uninstall-service >/dev/null 2>&1 || true
rm -f "$HOME/.local/bin/citizen-launcher" "$HOME/.local/lib/citizen-launcher/citizen-launcher"
rm -f "${XDG_DATA_HOME:-$HOME/.local/share}/applications/io.github.citizenlauncher.CitizenLauncher.desktop"
if command -v omarchy >/dev/null 2>&1; then omarchy plugin disable local.omarchy-citizen >/dev/null 2>&1 || true; fi
echo "Citizen Launcher entfernt. Spiele/PREFIX/Downloads bleiben erhalten."
