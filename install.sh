#!/usr/bin/env bash
# Citizen Launcher – Installer für Star Citizen unter Linux
#
#   bash <(wget -qO- https://github.com/b1tk1ll3r/sc-lnx-inst/releases/latest/download/install.sh)
#
# Optionen:
#   --user             Benutzerinstallation in ~/.local (ohne Root, auch für immutable Systeme)
#   --version X.Y.Z    bestimmte Version statt der neuesten installieren
#   --local            Binary aus diesem Source-Checkout installieren (backend/bin, nach ./build.sh)
#   --uninstall        Citizen Launcher entfernen (Prefix und Spieldaten bleiben erhalten)
#   -h, --help         diese Hilfe
#
# Ablauf: neueste Release ermitteln → SHA256SUMS.txt laden → Ed25519-Signatur
# mit dem unten eingebetteten Release-Schlüssel prüfen (OpenSSL) → passendes
# Paket laden → SHA-256 prüfen → über den Paketmanager der Distribution bzw.
# in ~/.local installieren. Ohne gültige Signatur wird nichts installiert.
#
# Der gesamte Code steckt in Funktionen und wird erst in der letzten Zeile
# ausgeführt: ein abgebrochener Download führt so nie halbe Befehle aus.

set -euo pipefail

REPO="${CITIZEN_LAUNCHER_REPO:-b1tk1ll3r/sc-lnx-inst}"

# Vertrauenswürdige Release-Schlüssel (Ed25519, SubjectPublicKeyInfo/PEM).
# Muss mit backend/internal/signing.PublicKeys übereinstimmen (wird getestet).
RELEASE_PUBKEYS=(
'-----BEGIN PUBLIC KEY-----
MCowBQYDK2VwAyEA2Ra7HkxMbqd4TlJFLosTXzzaC+sg0+CQgb/5VgQiG98=
-----END PUBLIC KEY-----'
)

APP_ID="io.github.citizenlauncher.CitizenLauncher"
BIN_DIR="$HOME/.local/bin"
DATA_HOME="${XDG_DATA_HOME:-$HOME/.local/share}"
CONFIG_HOME="${XDG_CONFIG_HOME:-$HOME/.config}"

if [[ -t 1 ]]; then
  C_B=$'\e[1m' C_G=$'\e[32m' C_Y=$'\e[33m' C_R=$'\e[31m' C_0=$'\e[0m'
else
  C_B='' C_G='' C_Y='' C_R='' C_0=''
fi

info() { printf '%s==>%s %s\n' "$C_B$C_G" "$C_0" "$*"; }
warn() { printf '%sHinweis:%s %s\n' "$C_Y" "$C_0" "$*" >&2; }
die() {
  printf '%sFehler:%s %s\n' "$C_R" "$C_0" "$*" >&2
  exit 1
}

usage() {
  cat <<EOF
Citizen Launcher Installer

  bash <(wget -qO- https://github.com/$REPO/releases/latest/download/install.sh) [Optionen]

  --user             Benutzerinstallation in ~/.local (ohne Root)
  --version X.Y.Z    bestimmte Version installieren
  --local            Binary aus diesem Source-Checkout installieren (nach ./build.sh)
  --uninstall        Citizen Launcher entfernen (Spieldaten bleiben erhalten)
  -h, --help         diese Hilfe
EOF
}

need() { command -v "$1" >/dev/null 2>&1 || die "'$1' wird benötigt. ${2:-}"; }

# fetch URL FILE – nur HTTPS, auch bei Weiterleitungen.
fetch() {
  local url="$1" out="$2"
  [[ "$url" == https://* ]] || die "Unsichere URL abgelehnt: $url"
  if command -v wget >/dev/null 2>&1; then
    wget -q --https-only -O "$out" "$url"
  elif command -v curl >/dev/null 2>&1; then
    curl -fsSL --proto '=https' --proto-redir '=https' --tlsv1.2 -o "$out" "$url"
  else
    die "wget oder curl wird benötigt."
  fi
}

root_run() {
  if [[ $EUID -eq 0 ]]; then
    "$@"
  elif command -v sudo >/dev/null 2>&1; then
    sudo "$@"
  elif command -v pkexec >/dev/null 2>&1; then
    pkexec "$@"
  else
    die "Für die Paketinstallation werden Administratorrechte benötigt (sudo oder pkexec)."
  fi
}

# --- Systemerkennung -------------------------------------------------------

FAMILY=other
IMMUTABLE=false
OS_NAME=Linux

detect_system() {
  [[ "$(uname -m)" == x86_64 ]] || die "Citizen Launcher unterstützt nur x86-64 (gefunden: $(uname -m))."
  local id="" like="" variant="" name=""
  if [[ -r /etc/os-release ]]; then
    # shellcheck disable=SC1091
    id="$(. /etc/os-release && printf '%s' "${ID:-}")"
    like="$(. /etc/os-release && printf '%s' "${ID_LIKE:-}")"
    variant="$(. /etc/os-release && printf '%s' "${VARIANT_ID:-}")"
    name="$(. /etc/os-release && printf '%s' "${PRETTY_NAME:-}")"
  fi
  OS_NAME="${name:-Linux}"
  case "$id" in
    debian|ubuntu|linuxmint|pop|elementary|zorin|kali|neon|tuxedo) FAMILY=debian ;;
    fedora|nobara|rhel|centos|rocky|almalinux|ultramarine) FAMILY=fedora ;;
    arch|manjaro|endeavouros|cachyos|garuda|steamos) FAMILY=arch ;;
    opensuse|opensuse-tumbleweed|opensuse-leap|opensuse-slowroll|sles|sled) FAMILY=suse ;;
    *)
      case " $like " in
        *" debian "*|*" ubuntu "*) FAMILY=debian ;;
        *" fedora "*|*" rhel "*) FAMILY=fedora ;;
        *" arch "*) FAMILY=arch ;;
        *" suse "*|*" opensuse "*) FAMILY=suse ;;
      esac
      ;;
  esac
  case "$id:$variant" in
    steamos:*|*:*silverblue*|*:*kinoite*|*:*sericea*|*:*onyx*|*:*atomic*|opensuse-microos:*|aeon:*|kalpa:*|bazzite:*|bluefin:*|aurora:*) IMMUTABLE=true ;;
  esac
  [[ -e /run/ostree-booted || -e /run/transactional-update ]] && IMMUTABLE=true
  return 0
}

native_package_installed() {
  if command -v dpkg-query >/dev/null 2>&1 && dpkg-query -W -f='${Status}' citizen-launcher 2>/dev/null | grep -q 'install ok installed'; then
    return 0
  fi
  if command -v rpm >/dev/null 2>&1 && rpm -q citizen-launcher >/dev/null 2>&1; then
    return 0
  fi
  if command -v pacman >/dev/null 2>&1 && pacman -Q citizen-launcher >/dev/null 2>&1; then
    return 0
  fi
  return 1
}

# --- Release, Signatur, Prüfsummen -----------------------------------------

resolve_version() {
  local want="$1" json
  if [[ -z "$want" ]]; then
    json="$WORK/release.json"
    fetch "https://api.github.com/repos/$REPO/releases/latest" "$json" ||
      die "Neueste Release von $REPO konnte nicht ermittelt werden."
    want="$(sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"v\{0,1\}\([^"]*\)".*/\1/p' "$json" | head -n1)"
  fi
  want="${want#v}"
  [[ "$want" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || die "Ungültige Version: '$want'"
  printf '%s\n' "$want"
}

# verify_manifest SUMS SIG – Ed25519 über die exakten Bytes von SHA256SUMS.txt.
verify_manifest() {
  local sums="$1" sig="$2" key i=0
  need openssl "Bitte das Paket 'openssl' installieren."
  base64 -d < "$sig" > "$WORK/sig.bin" 2>/dev/null || die "Signaturdatei ist beschädigt."
  for key in "${RELEASE_PUBKEYS[@]}"; do
    i=$((i + 1))
    printf '%s\n' "$key" > "$WORK/release-key-$i.pem"
    if openssl pkeyutl -verify -pubin -inkey "$WORK/release-key-$i.pem" -rawin \
         -in "$sums" -sigfile "$WORK/sig.bin" >/dev/null 2>&1; then
      return 0
    fi
  done
  if ! openssl pkeyutl -help 2>&1 | grep -q -- '-rawin'; then
    die "OpenSSL ist zu alt für Ed25519-Prüfungen (benötigt OpenSSL 3)."
  fi
  die "Die Signatur von SHA256SUMS.txt ist UNGÜLTIG. Abbruch – es wurde nichts installiert."
}

# verify_asset SUMS FILE NAME
verify_asset() {
  local sums="$1" file="$2" name="$3" want got
  want="$(awk -v n="$name" '{f=$2; sub(/^\*/, "", f)} f == n {print tolower($1); exit}' "$sums")"
  [[ "$want" =~ ^[0-9a-f]{64}$ ]] || die "$name ist nicht in der signierten SHA256SUMS.txt aufgeführt."
  got="$(sha256sum "$file" | awk '{print $1}')"
  [[ "$got" == "$want" ]] || die "SHA-256 von $name stimmt nicht. Abbruch – es wurde nichts installiert."
}

asset_name() {
  local mode="$1" v="$2"
  case "$mode" in
    deb) printf 'citizen-launcher_%s_amd64.deb\n' "$v" ;;
    rpm) printf 'citizen-launcher-%s-1.linux.x86_64.rpm\n' "$v" ;;
    arch) printf 'citizen-launcher-%s-1-x86_64.pkg.tar.zst\n' "$v" ;;
    user) printf 'citizen-launcher-%s-linux-amd64.tar.gz\n' "$v" ;;
  esac
}

download_verified() {
  local version="$1" name="$2" base
  base="https://github.com/$REPO/releases/download/v$version"
  info "Lade signierte Prüfsummen für Version $version …"
  fetch "$base/SHA256SUMS.txt" "$WORK/SHA256SUMS.txt" || die "SHA256SUMS.txt fehlt in Release v$version."
  fetch "$base/SHA256SUMS.txt.sig" "$WORK/SHA256SUMS.txt.sig" || die "Release v$version ist nicht signiert."
  verify_manifest "$WORK/SHA256SUMS.txt" "$WORK/SHA256SUMS.txt.sig"
  info "Signatur gültig. Lade $name …"
  fetch "$base/$name" "$WORK/$name" || die "Download von $name fehlgeschlagen."
  verify_asset "$WORK/SHA256SUMS.txt" "$WORK/$name" "$name"
  info "SHA-256 geprüft."
}

# --- Installation ------------------------------------------------------------

install_native() {
  local mode="$1" file="$2"
  # Paketmanager (z. B. APT als _apt) müssen die Datei lesen können.
  chmod 0755 "$WORK"
  chmod 0644 "$file"
  case "$mode" in
    deb) root_run apt-get install -y "$file" ;;
    rpm)
      if command -v dnf5 >/dev/null 2>&1; then
        root_run dnf5 install -y "$file"
      elif command -v dnf >/dev/null 2>&1; then
        root_run dnf install -y "$file"
      elif command -v zypper >/dev/null 2>&1; then
        root_run zypper --non-interactive install --allow-unsigned-rpm "$file"
      else
        root_run rpm -Uvh --replacepkgs "$file"
      fi
      ;;
    arch) root_run pacman -U --noconfirm "$file" ;;
  esac
}

# install_user BINARY [ICON]
install_user() {
  local bin="$1" icon="${2:-}" app_dir icon_dir warnings=()
  [[ $EUID -ne 0 ]] || die "Die Benutzerinstallation bitte als normaler Benutzer (ohne sudo) ausführen."
  if native_package_installed && ! $IMMUTABLE && [[ "${CITIZEN_LAUNCHER_ALLOW_USER_SHADOW:-0}" != 1 ]]; then
    die "Citizen Launcher ist bereits als Systempaket installiert; eine ~/.local-Kopie würde es überschatten. Updates kommen automatisch über das Paket."
  fi
  app_dir="$DATA_HOME/applications"
  icon_dir="$DATA_HOME/icons/hicolor/scalable/apps"
  mkdir -p "$BIN_DIR" "$app_dir" "$icon_dir"
  install -m755 "$bin" "$BIN_DIR/citizen-launcher.new" || die "Kopieren nach $BIN_DIR fehlgeschlagen."
  mv -f "$BIN_DIR/citizen-launcher.new" "$BIN_DIR/citizen-launcher" || die "Kopieren nach $BIN_DIR fehlgeschlagen."
  [[ -n "$icon" && -f "$icon" ]] && install -m644 "$icon" "$icon_dir/citizen-launcher.svg"
  cat > "$app_dir/$APP_ID.desktop" <<EOF
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
EOF
  if ! "$BIN_DIR/citizen-launcher" prepare-system; then
    warnings+=("Linux-Systemlimits konnten nicht sofort gesetzt werden; der Launcher bietet das beim Setup erneut an.")
  fi
  if ! "$BIN_DIR/citizen-launcher" autopilot enable; then
    warnings+=("Autopilot konnte nicht aktiviert werden; das geht später in der GUI.")
  fi
  command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database "$app_dir" >/dev/null 2>&1 || true
  command -v gtk-update-icon-cache >/dev/null 2>&1 && gtk-update-icon-cache -f "$DATA_HOME/icons/hicolor" >/dev/null 2>&1 || true
  case ":$PATH:" in
    *":$BIN_DIR:"*) ;;
    *) warnings+=("$BIN_DIR ist nicht im PATH; Start über das App-Menü oder $BIN_DIR/citizen-launcher gui.") ;;
  esac
  if ((${#warnings[@]})); then
    printf '%s\n' "${warnings[@]}" | while IFS= read -r w; do warn "$w"; done
  fi
}

uninstall() {
  if native_package_installed; then
    info "Entferne Systempaket …"
    if command -v apt-get >/dev/null 2>&1 && command -v dpkg-query >/dev/null 2>&1 && dpkg-query -W citizen-launcher >/dev/null 2>&1; then
      root_run apt-get remove -y citizen-launcher
    elif command -v pacman >/dev/null 2>&1 && pacman -Q citizen-launcher >/dev/null 2>&1; then
      root_run pacman -R --noconfirm citizen-launcher
    elif command -v dnf5 >/dev/null 2>&1; then
      root_run dnf5 remove -y citizen-launcher
    elif command -v dnf >/dev/null 2>&1; then
      root_run dnf remove -y citizen-launcher
    elif command -v zypper >/dev/null 2>&1; then
      root_run zypper --non-interactive remove citizen-launcher
    else
      root_run rpm -e citizen-launcher
    fi
  fi
  if [[ -x "$BIN_DIR/citizen-launcher" ]]; then
    info "Entferne Benutzerinstallation …"
    "$BIN_DIR/citizen-launcher" autopilot disable >/dev/null 2>&1 || true
  fi
  systemctl --user disable --now citizen-launcher-maintenance.timer >/dev/null 2>&1 || true
  rm -f "$CONFIG_HOME/systemd/user/citizen-launcher-maintenance.service" \
        "$CONFIG_HOME/systemd/user/citizen-launcher-maintenance.timer"
  systemctl --user daemon-reload >/dev/null 2>&1 || true
  rm -f "$BIN_DIR/citizen-launcher" "$HOME/.local/lib/citizen-launcher/citizen-launcher" \
        "$DATA_HOME/applications/$APP_ID.desktop" \
        "$DATA_HOME/applications/citizen-launcher-star-citizen.desktop" \
        "$DATA_HOME/icons/hicolor/scalable/apps/citizen-launcher.svg" \
        "$DATA_HOME/citizen-launcher/bin/star-citizen-launch"
  command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database "$DATA_HOME/applications" >/dev/null 2>&1 || true
  info "Citizen Launcher entfernt."
  echo "Wine-Prefix und Spieldaten (Standard: ~/Games/star-citizen), Konfiguration und Logs bleiben erhalten."
}

main() {
  local mode="" version="" local_build=false do_uninstall=false
  while (($#)); do
    case "$1" in
      --user) mode=user ;;
      --version) version="${2:-}"; shift ;;
      --version=*) version="${1#*=}" ;;
      --local) local_build=true; mode=user ;;
      --uninstall) do_uninstall=true ;;
      -h|--help) usage; return 0 ;;
      *) die "Unbekannte Option: $1 (siehe --help)" ;;
    esac
    shift
  done

  WORK="$(mktemp -d)"
  trap 'rm -rf "$WORK"' EXIT

  if $do_uninstall; then
    uninstall
    return 0
  fi

  detect_system

  if $local_build; then
    local root
    root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
    [[ -x "$root/backend/bin/citizen-launcher" ]] || die "--local braucht einen Source-Checkout mit gebautem Backend (./build.sh)."
    info "Installiere lokalen Build aus $root …"
    install_user "$root/backend/bin/citizen-launcher" "$root/packaging/icons/citizen-launcher.svg" || die "Benutzerinstallation fehlgeschlagen."
    info "Fertig. Start über das App-Menü oder: citizen-launcher gui"
    return 0
  fi

  if [[ -z "$mode" ]]; then
    if $IMMUTABLE; then
      mode=user
      info "Immutable System erkannt – Installation im Benutzerkonto, das Basis-Image bleibt unangetastet."
    else
      case "$FAMILY" in
        debian) command -v apt-get >/dev/null 2>&1 && mode=deb ;;
        fedora|suse) command -v rpm >/dev/null 2>&1 && mode=rpm ;;
        arch) command -v pacman >/dev/null 2>&1 && mode=arch ;;
      esac
      [[ -n "$mode" ]] || mode=user
    fi
  fi

  need sha256sum
  need base64
  need tar
  # Explizite Fehlerbehandlung: set -e greift in Kommandosubstitutionen nicht immer.
  version="$(resolve_version "$version")" || exit 1
  local name
  name="$(asset_name "$mode" "$version")"
  info "Citizen Launcher $version für $OS_NAME ($mode)"
  download_verified "$version" "$name"

  if [[ "$mode" == user ]]; then
    mkdir -p "$WORK/unpack"
    tar -xzf "$WORK/$name" -C "$WORK/unpack" --no-same-owner --no-same-permissions || die "Tarball konnte nicht entpackt werden."
    [[ -f "$WORK/unpack/citizen-launcher" ]] || die "Tarball enthält kein citizen-launcher."
    install_user "$WORK/unpack/citizen-launcher" "$WORK/unpack/citizen-launcher.svg" || die "Benutzerinstallation fehlgeschlagen."
  else
    install_native "$mode" "$WORK/$name" || die "Paketinstallation fehlgeschlagen."
  fi

  info "Citizen Launcher $version installiert."
  echo "Start über das App-Menü oder mit: citizen-launcher gui"
}

# Nur ausführen, wenn direkt gestartet (nicht beim Einbinden in Tests).
if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
