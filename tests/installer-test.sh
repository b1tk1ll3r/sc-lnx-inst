#!/usr/bin/env bash
# Exercises install.sh against a local fake release signed with a throw-away
# Ed25519 key: signature/checksum verification, tamper detection, user-mode
# install and native-package dispatch. Needs bash, openssl 3, sha256sum, tar.
set -euo pipefail
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

VERSION=9.8.7
REL="$TMP/release"
mkdir -p "$REL" "$TMP/bin" "$TMP/home"

# Fake launcher binary inside the user tarball.
cat > "$TMP/citizen-launcher" <<'EOF'
#!/usr/bin/env bash
case "${1:-}" in
  --version) echo 9.8.7 ;;
  *) exit 0 ;;
esac
EOF
chmod 755 "$TMP/citizen-launcher"
cp "$ROOT/packaging/icons/citizen-launcher.svg" "$TMP/citizen-launcher.svg"
tar -C "$TMP" -czf "$REL/citizen-launcher-$VERSION-linux-amd64.tar.gz" citizen-launcher citizen-launcher.svg
echo fake-deb > "$REL/citizen-launcher_${VERSION}_amd64.deb"

(cd "$REL" && sha256sum -- * > SHA256SUMS.txt)
openssl genpkey -algorithm ed25519 -out "$TMP/key.pem" 2>/dev/null
openssl pkey -in "$TMP/key.pem" -pubout -out "$TMP/pub.pem"
sign() { openssl pkeyutl -sign -inkey "$TMP/key.pem" -rawin -in "$1" | base64 | tr -d '\n' > "$1.sig"; }
sign "$REL/SHA256SUMS.txt"

# Load install.sh functions without running main.
# shellcheck source=../install.sh
source "$ROOT/install.sh"
set +e
RELEASE_PUBKEYS=("$(cat "$TMP/pub.pem")")
fetch() { cp "$REL/${1##*/}" "$2"; }
detect_system() { FAMILY=other; IMMUTABLE=false; OS_NAME=Test; }
export HOME="$TMP/home" XDG_DATA_HOME="$TMP/home/.local/share" XDG_CONFIG_HOME="$TMP/home/.config"
BIN_DIR="$HOME/.local/bin" DATA_HOME="$XDG_DATA_HOME" CONFIG_HOME="$XDG_CONFIG_HOME"

expect_ok() { local d="$1"; shift; ("$@") >"$TMP/out" 2>&1 || { cat "$TMP/out"; echo "FAIL: $d" >&2; exit 1; }; echo "ok   $d"; }
expect_fail() { local d="$1"; shift; if ("$@") >"$TMP/out" 2>&1; then cat "$TMP/out"; echo "FAIL (unexpected success): $d" >&2; exit 1; fi; echo "ok   $d  [$(tail -n1 "$TMP/out")]"; }

[[ "$(asset_name rpm 1.2.3)" == citizen-launcher-1.2.3-1.linux.x86_64.rpm ]]
[[ "$(asset_name arch 1.2.3)" == citizen-launcher-1.2.3-1-x86_64.pkg.tar.zst ]]

expect_ok   "user install with valid signature" main --user --version "$VERSION"
[[ -x "$HOME/.local/bin/citizen-launcher" ]]
[[ -f "$XDG_DATA_HOME/icons/hicolor/scalable/apps/citizen-launcher.svg" ]]
grep -q "^Exec=$HOME/.local/bin/citizen-launcher gui$" "$XDG_DATA_HOME/applications/io.github.citizenlauncher.CitizenLauncher.desktop"

expect_fail "invalid version string rejected" main --user --version '1.2.3;rm'

# Tampered manifest (signature no longer matches).
cp "$REL/SHA256SUMS.txt" "$TMP/sums.orig"
echo "0000000000000000000000000000000000000000000000000000000000000000  extra" >> "$REL/SHA256SUMS.txt"
expect_fail "tampered SHA256SUMS.txt rejected" main --user --version "$VERSION"
cp "$TMP/sums.orig" "$REL/SHA256SUMS.txt"

# Swapped asset (manifest valid, file content differs).
cp "$REL/citizen-launcher_${VERSION}_amd64.deb" "$TMP/deb.orig"
echo evil > "$REL/citizen-launcher_${VERSION}_amd64.deb"
detect_system() { FAMILY=debian; IMMUTABLE=false; OS_NAME=Test; }
printf '#!/usr/bin/env bash\necho "$@" >> "%s"\n' "$TMP/apt.log" > "$TMP/bin/apt-get"
chmod 755 "$TMP/bin/apt-get"
PATH="$TMP/bin:$PATH"
root_run() { "$@"; }
expect_fail "swapped package rejected" main --version "$VERSION"
[[ ! -s "$TMP/apt.log" ]] || { echo "FAIL: apt-get ran for an unverified package" >&2; exit 1; }
cp "$TMP/deb.orig" "$REL/citizen-launcher_${VERSION}_amd64.deb"

expect_ok   "native .deb dispatched to apt-get" main --version "$VERSION"
grep -q "install -y .*citizen-launcher_${VERSION}_amd64.deb" "$TMP/apt.log"

# Signature by a different key.
openssl genpkey -algorithm ed25519 -out "$TMP/key.pem" 2>/dev/null
sign "$REL/SHA256SUMS.txt"
expect_fail "signature by untrusted key rejected" main --user --version "$VERSION"

echo 'Installer verification: OK'
