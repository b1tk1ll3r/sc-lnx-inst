#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd -- "$(dirname -- "$0")" && pwd)"
BACK="$ROOT/bin/citizen-launcher"
TEST="$(mktemp -d -t omarchy-citizen-updater.XXXXXX)"
trap 'rm -rf "$TEST"' EXIT

mkdir -p "$TEST/home/.config/omarchy/plugins" "$TEST/bin"
export HOME="$TEST/home"
export XDG_CONFIG_HOME="$HOME/.config"
export XDG_STATE_HOME="$HOME/.local/state"
export PATH="$TEST/bin:/usr/bin:/bin"

printf '#!/usr/bin/env bash\nexit 0\n' > "$TEST/bin/systemctl"
printf '#!/usr/bin/env bash\nexit 0\n' > "$TEST/bin/omarchy-shell"
chmod +x "$TEST/bin/systemctl" "$TEST/bin/omarchy-shell"

git init --bare -q "$TEST/origin.git"
git clone -q "$TEST/origin.git" "$TEST/publisher"
git -C "$TEST/publisher" config user.email test@example.com
git -C "$TEST/publisher" config user.name Test
mkdir -p "$TEST/publisher/backend/bin"
cp "$BACK" "$TEST/publisher/backend/bin/"
printf '%s\n' '{"schemaVersion":1,"id":"local.omarchy-citizen","name":"Test","version":"0.6.0","kinds":["bar-widget"],"entryPoints":{"barWidget":"BarWidget.qml"},"barWidget":{"displayName":"Test","category":"Gaming","allowMultiple":false}}' > "$TEST/publisher/manifest.json"
printf '%s\n' 'import QtQuick; Item {}' > "$TEST/publisher/BarWidget.qml"
git -C "$TEST/publisher" add .
git -C "$TEST/publisher" commit -qm v1
git -C "$TEST/publisher" push -qu origin HEAD:master

git clone -q "$TEST/origin.git" "$HOME/.config/omarchy/plugins/local.omarchy-citizen"

cat > "$TEST/bin/omarchy" <<'SCRIPT'
#!/usr/bin/env bash
set -e
PLUGIN="$HOME/.config/omarchy/plugins/local.omarchy-citizen"
if [[ "$1 $2" == "plugin validate" ]]; then
  [[ -f "$3/manifest.json" && -f "$3/BarWidget.qml" ]]
  exit 0
fi
if [[ "$1 $2 $3" == "plugin update local.omarchy-citizen" ]]; then
  git -C "$PLUGIN" pull --ff-only -q
  exit 0
fi
exit 2
SCRIPT
chmod +x "$TEST/bin/omarchy"

[[ "$("$BACK" status | sed -n 's/^update=//p')" == "current" ]]
printf 'v2\n' > "$TEST/publisher/new-file.txt"
git -C "$TEST/publisher" add new-file.txt
git -C "$TEST/publisher" commit -qm v2
git -C "$TEST/publisher" push -q
[[ "$("$BACK" check | sed -n 's/^update=//p')" == "available" ]]
"$BACK" update
[[ "$("$BACK" status | sed -n 's/^update=//p')" == "current" ]]
"$BACK" auto enable >/dev/null
[[ "$("$BACK" status | sed -n 's/^auto_apply=//p')" == "true" ]]

git -C "$HOME/.config/omarchy/plugins/local.omarchy-citizen" remote set-url origin "$TEST/changed.git"
set +e
CHECK_OUTPUT="$("$HOME/.local/lib/citizen-launcher/citizen-launcher" check 2>&1)"
RC=$?
set -e
[[ "$RC" -ne 0 ]]
grep -q 'update=blocked-remote-changed' <<<"$CHECK_OUTPUT"

echo 'integration test: PASS'
