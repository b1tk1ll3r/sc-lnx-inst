#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
BIN_SRC="$ROOT/backend/bin/citizen-launcher"
BIN_DIR="$HOME/.local/bin"
LIB_DIR="$HOME/.local/lib/citizen-launcher"
APP_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/applications"

[[ -x "$BIN_SRC" ]] || { echo "Backend fehlt. Zuerst ./build.sh ausführen." >&2; exit 1; }
mkdir -p "$BIN_DIR" "$LIB_DIR" "$APP_DIR"
install -m755 "$BIN_SRC" "$BIN_DIR/citizen-launcher"
install -m755 "$BIN_SRC" "$LIB_DIR/citizen-launcher"
cat > "$APP_DIR/io.github.citizenlauncher.CitizenLauncher.desktop" <<EOF
[Desktop Entry]
Name=Citizen Launcher
Comment=Star Citizen für Linux – Setup, Start, Updates und Reparatur
Exec=$BIN_DIR/citizen-launcher gui
Terminal=false
Type=Application
Categories=Game;
StartupNotify=true
Keywords=Star Citizen;RSI;Wine;Gaming;
EOF
"$BIN_DIR/citizen-launcher" autopilot enable || true
if command -v update-desktop-database >/dev/null 2>&1; then update-desktop-database "$APP_DIR" >/dev/null 2>&1 || true; fi
echo "Citizen Launcher installiert. Starte ihn über das App-Menü oder: citizen-launcher gui"
