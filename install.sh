#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
BIN_SRC="$ROOT/backend/bin/citizen-launcher"
BIN_DIR="$HOME/.local/bin"

system_package_installed=false
if command -v dpkg-query >/dev/null 2>&1 && dpkg-query -W -f='${Status}' citizen-launcher 2>/dev/null | grep -q 'install ok installed'; then
  system_package_installed=true
elif command -v rpm >/dev/null 2>&1 && rpm -q citizen-launcher >/dev/null 2>&1; then
  system_package_installed=true
elif command -v pacman >/dev/null 2>&1 && pacman -Q citizen-launcher >/dev/null 2>&1; then
  system_package_installed=true
fi

immutable=false
[[ -e /run/ostree-booted || -e /run/transactional-update ]] && immutable=true
if [[ -r /etc/os-release ]]; then
  os_id="$(sed -n 's/^ID=//p' /etc/os-release | tr -d '"' | head -n1)"
  case "$os_id" in steamos|opensuse-microos|aeon|kalpa) immutable=true;; esac
fi

if $system_package_installed && ! $immutable && [[ "${CITIZEN_LAUNCHER_ALLOW_USER_SHADOW:-0}" != 1 ]]; then
  echo "Eine native Citizen-Launcher-Paketinstallation ist bereits vorhanden." >&2
  echo "Die ~/.local-Version wird nicht darübergelegt, damit keine alte Version /usr/bin überschattet." >&2
  echo "Bitte ./INSTALLIEREN.sh oder das native .deb/.rpm/.pkg.tar.zst für Updates verwenden." >&2
  exit 2
fi
APP_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/applications"
ICON_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/icons/hicolor/scalable/apps"

[[ -x "$BIN_SRC" ]] || { echo "Backend fehlt. Zuerst ./build.sh ausführen." >&2; exit 1; }
mkdir -p "$BIN_DIR" "$APP_DIR" "$ICON_DIR"
install -m755 "$BIN_SRC" "$BIN_DIR/citizen-launcher"
install -m644 "$ROOT/packaging/icons/citizen-launcher.svg" "$ICON_DIR/citizen-launcher.svg"
cat > "$APP_DIR/io.github.citizenlauncher.CitizenLauncher.desktop" <<EOF2
[Desktop Entry]
Name=Citizen Launcher
Comment=Star Citizen für Linux – Setup, Start, Updates und Reparatur
Exec=$BIN_DIR/citizen-launcher gui
TryExec=$BIN_DIR/citizen-launcher
Icon=citizen-launcher
Terminal=false
Type=Application
Categories=Game;
StartupNotify=true
StartupWMClass=CitizenLauncher
Keywords=Star Citizen;RSI;Wine;Gaming;
EOF2

warnings=()
if ! "$BIN_DIR/citizen-launcher" prepare-system; then
  warnings+=("Linux-Systemlimits konnten nicht sofort gesetzt werden. Citizen Launcher bietet die Vorbereitung beim Setup erneut an.")
fi
if ! "$BIN_DIR/citizen-launcher" autopilot enable; then
  warnings+=("Autopilot konnte nicht automatisch aktiviert werden; der Launcher funktioniert trotzdem und kann ihn später aktivieren.")
fi
if command -v update-desktop-database >/dev/null 2>&1; then update-desktop-database "$APP_DIR" >/dev/null 2>&1 || true; fi
if command -v gtk-update-icon-cache >/dev/null 2>&1; then gtk-update-icon-cache -f "${XDG_DATA_HOME:-$HOME/.local/share}/icons/hicolor" >/dev/null 2>&1 || true; fi

echo "Citizen Launcher installiert. Starte ihn über das App-Menü oder: citizen-launcher gui"
if ((${#warnings[@]})); then
  printf '\nHinweise:\n'
  printf '  - %s\n' "${warnings[@]}"
fi
