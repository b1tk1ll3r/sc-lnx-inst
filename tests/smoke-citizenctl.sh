#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
mkdir -p "$TMP/bin" "$TMP/home/Downloads"
CALLS="$TMP/calls.log"
CTL="$ROOT/integrations/omarchy/citizenctl"

# Fake launcher on PATH; citizenctl prefers the installed citizen-launcher.
cat > "$TMP/bin/citizen-launcher" <<'BACKEND'
#!/usr/bin/env bash
set -u
printf '%s\n' "$*" >> "${FAKE_CALLS:?}"
case "${1:-status}" in
  game-status)
    cat <<STATUS
backend_version=0.8.1
health=${FAKE_HEALTH:-install}
hardware=ready
hardware_reason=
gpu=Test GPU
vulkan=ready
cpu_avx=true
ram_gib=32
combined_gib=40
disk_free_gib=300
prefix=$HOME/Games/star-citizen
prefix_state=${FAKE_PREFIX_STATE:-missing}
launcher_state=${FAKE_LAUNCHER_STATE:-missing}
game_state=${FAKE_GAME_STATE:-missing}
wine_version=11.14-1
dxvk_version=v2.7
lug_version=v4.16
rsi_installer=RSI-Setup.exe
autopilot=true
STATUS
    ;;
  status|check)
    cat <<STATUS
backend_version=0.8.1
managed=git
auto_apply=true
auto_maintain=true
lug_version=v4.16
wine_version=11.14-1
dxvk_version=v2.7
last_maintenance=now
maintenance_result=ok
update=current
remote=https://example.invalid/repo.git
trusted_remote=https://example.invalid/repo.git
branch=main
local_commit=abc
remote_commit=abc
dirty=false
last_check=now
last_update=
last_result=ok
STATUS
    ;;
  doctor) echo 'runner=test'; echo 'wine_selftest=ok' ;;
  autopilot|auto|maintain|update|game-install|game-launch|game-repair) : ;;
  *) echo "unexpected fake backend command: $*" >&2; exit 3 ;;
esac
BACKEND

cat > "$TMP/bin/xdg-terminal-exec" <<'TERM'
#!/usr/bin/env bash
printf 'terminal:%s\n' "$*" >> "${FAKE_CALLS:?}"
exit 0
TERM
cat > "$TMP/bin/uwsm-app" <<'UWSM'
#!/usr/bin/env bash
[[ "${1:-}" == "--" ]] && shift
exec "$@"
UWSM
cat > "$TMP/bin/notify-send" <<'NOTIFY'
#!/usr/bin/env bash
exit 0
NOTIFY
cat > "$TMP/bin/xdg-user-dir" <<'XDG'
#!/usr/bin/env bash
printf '%s/Downloads\n' "$HOME"
XDG
chmod +x "$TMP/bin/"*

export HOME="$TMP/home"
export XDG_CONFIG_HOME="$HOME/.config"
export XDG_DATA_HOME="$HOME/.local/share"
export XDG_STATE_HOME="$HOME/.local/state"
export XDG_CACHE_HOME="$HOME/.cache"
export PATH="$TMP/bin:/usr/bin:/bin"
export FAKE_CALLS="$CALLS"

# 1. Status must be fully routed through the Go backend without missing shell functions.
out="$(bash "$CTL" status 2>&1)"
grep -q '^plugin_version=' <<<"$out"
grep -q '^health=install$' <<<"$out"
grep -q '^managed_wine_version=11.14-1$' <<<"$out"
! grep -qi 'command not found' <<<"$out"

# 2. The complete setup terminal action must call all owned backend stages.
: > "$CALLS"
bash "$CTL" install-stack-terminal >/dev/null 2>&1
grep -q '^autopilot enable$' "$CALLS"
grep -q '^doctor$' "$CALLS"
grep -q '^game-install$' "$CALLS"
grep -q '^maintain$' "$CALLS"

# 3. Primary setup routing must open the visible setup terminal.
: > "$CALLS"
FAKE_HEALTH=install bash "$CTL" primary >/dev/null 2>&1
grep -q 'terminal:.*install-stack-terminal' "$CALLS"

# 4. A ready installation launches through the Go backend directly.
: > "$CALLS"
FAKE_HEALTH=ready FAKE_PREFIX_STATE=ready FAKE_LAUNCHER_STATE=ready FAKE_GAME_STATE=ready \
  bash "$CTL" primary >/dev/null 2>&1
grep -q '^game-launch$' "$CALLS"

# 5. Repair routing remains visible.
: > "$CALLS"
FAKE_HEALTH=repair bash "$CTL" primary >/dev/null 2>&1
grep -q 'terminal:.*repair-stack-terminal' "$CALLS"

# 6. Plugin updater wrapper must not collide with the combined status function.
: > "$CALLS"
bash "$CTL" plugin-update-check >/dev/null 2>&1
grep -q '^check$' "$CALLS"

echo 'citizenctl smoke tests: OK'
