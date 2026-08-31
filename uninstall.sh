#!/usr/bin/env bash
set -euo pipefail
BIN="$HOME/.local/bin/citizen-launcher"
DATA_HOME="${XDG_DATA_HOME:-$HOME/.local/share}"
CONFIG_HOME="${XDG_CONFIG_HOME:-$HOME/.config}"
if [[ -x "$BIN" ]]; then
  "$BIN" autopilot disable >/dev/null 2>&1 || true
fi
systemctl --user disable --now citizen-launcher-maintenance.timer >/dev/null 2>&1 || true
rm -f "$CONFIG_HOME/systemd/user/citizen-launcher-maintenance.service" "$CONFIG_HOME/systemd/user/citizen-launcher-maintenance.timer"
systemctl --user daemon-reload >/dev/null 2>&1 || true
rm -f "$HOME/.local/bin/citizen-launcher" "$HOME/.local/lib/citizen-launcher/citizen-launcher"
rm -f "$DATA_HOME/applications/io.github.citizenlauncher.CitizenLauncher.desktop"
rm -f "$DATA_HOME/applications/citizen-launcher-star-citizen.desktop"
rm -f "$DATA_HOME/icons/hicolor/scalable/apps/citizen-launcher.svg"
rm -f "$DATA_HOME/citizen-launcher/bin/star-citizen-launch"
if command -v update-desktop-database >/dev/null 2>&1; then update-desktop-database "$DATA_HOME/applications" >/dev/null 2>&1 || true; fi
if command -v omarchy >/dev/null 2>&1; then omarchy plugin disable local.omarchy-citizen >/dev/null 2>&1 || true; fi
echo "Citizen Launcher entfernt. Star-Citizen-Prefix, Spieldateien, Konfiguration und Support-Logs bleiben erhalten."
