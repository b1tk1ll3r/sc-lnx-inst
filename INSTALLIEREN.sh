#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
VERSION="$(tr -d '[:space:]' < "$ROOT/VERSION")"

. /etc/os-release 2>/dev/null || true
id="${ID:-linux}"
like=" ${ID_LIKE:-} "
variant="${VARIANT_ID:-}"
immutable=false
case "$id:$variant" in
  steamos:*|*:*silverblue*|*:*kinoite*|*:*sericea*|*:*onyx*|*:*atomic*|opensuse-microos:*|aeon:*|kalpa:*) immutable=true ;;
esac
[[ -e /run/ostree-booted ]] && immutable=true

root_run() {
  if [[ $EUID -eq 0 ]]; then "$@"; return; fi
  if command -v pkexec >/dev/null 2>&1; then pkexec "$@"; return; fi
  if command -v sudo >/dev/null 2>&1; then sudo "$@"; return; fi
  echo "Für eine native Paketinstallation werden Administratorrechte benötigt." >&2
  return 1
}

family=other
case "$id" in
  debian|ubuntu|linuxmint|pop|elementary|zorin|kali|neon|tuxedo) family=debian ;;
  fedora|nobara|rhel|centos|rocky|almalinux|ultramarine) family=fedora ;;
  arch|manjaro|endeavouros|cachyos|garuda|steamos) family=arch ;;
  opensuse|opensuse-tumbleweed|opensuse-leap|opensuse-slowroll|sles|sled) family=suse ;;
  *)
    [[ "$like" == *" debian "* || "$like" == *" ubuntu "* ]] && family=debian
    [[ "$like" == *" fedora "* || "$like" == *" rhel "* ]] && family=fedora
    [[ "$like" == *" arch "* ]] && family=arch
    [[ "$like" == *" suse "* || "$like" == *" opensuse "* ]] && family=suse
    ;;
esac

if $immutable; then
  echo "Immutable Linux-Variante erkannt ($id${variant:+/$variant})."
  echo "Citizen Launcher wird sicher im Benutzerkonto installiert; das Basis-Image bleibt unangetastet."
  exec bash "$ROOT/install.sh"
fi

case "$family" in
  debian)
    pkg="$ROOT/dist/citizen-launcher_${VERSION}_amd64.deb"
    if [[ -s "$pkg" ]] && command -v apt-get >/dev/null 2>&1; then
      echo "Debian/Ubuntu-Familie erkannt – installiere natives .deb …"
      root_run apt-get install -y "$pkg"
      exit
    fi
    ;;
  fedora|suse)
    pkg="$ROOT/dist/citizen-launcher-${VERSION}-1.linux.x86_64.rpm"
    if [[ -s "$pkg" ]] && command -v rpm >/dev/null 2>&1; then
      echo "RPM-Familie erkannt – installiere natives RPM …"
      if [[ "$family" == fedora ]] && { command -v dnf5 >/dev/null 2>&1 || command -v dnf >/dev/null 2>&1; }; then
        pm=dnf; command -v dnf5 >/dev/null 2>&1 && pm=dnf5
        root_run "$pm" -y install "$pkg"
      elif command -v zypper >/dev/null 2>&1; then
        # Install stable dependencies through the distro solver first. The local
        # RPM itself is integrity-checked by the release digest during updates.
        root_run zypper --non-interactive install ca-certificates tar curl unzip xz polkit util-linux || true
        root_run rpm -Uvh --replacepkgs "$pkg"
      else
        root_run rpm -Uvh --replacepkgs "$pkg"
      fi
      exit
    fi
    ;;
  arch)
    pkg="$ROOT/dist/citizen-launcher-${VERSION}-1-x86_64.pkg.tar.zst"
    if [[ -s "$pkg" ]] && command -v pacman >/dev/null 2>&1; then
      echo "Arch-Familie erkannt – installiere natives pacman-Paket …"
      root_run pacman -U --noconfirm "$pkg"
      exit
    fi
    ;;
esac

echo "Kein passendes natives Paket im Projektordner gefunden."
echo "Installiere die portable, vollständig updatefähige Benutzer-Version …"
exec bash "$ROOT/install.sh"
