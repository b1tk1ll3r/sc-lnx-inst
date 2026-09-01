#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
for f in \
  "$ROOT/install.sh" "$ROOT/uninstall.sh" "$ROOT/install-omarchy.sh" "$ROOT/INSTALLIEREN.sh" \
  "$ROOT/build.sh" "$ROOT/packaging/build-deb.sh" "$ROOT/packaging/build-rpm.sh" \
  "$ROOT/packaging/build-arch.sh" "$ROOT/packaging/build-tarball.sh" "$ROOT/packaging/build-all.sh" \
  "$ROOT/scripts/gitea-release.sh" "$ROOT/tests/gitea-release-helper.sh" "$ROOT/tests/mode-independence.sh" "$ROOT/tests/rpm-ci-footprint.sh"; do
  bash -n "$f"
done
(cd "$ROOT/backend" && gofmt -w cmd/citizen-launcher && go test ./... && go vet ./...)
bash "$ROOT/build.sh"
[[ "$("$ROOT/backend/bin/citizen-launcher" --version)" == "$(tr -d '[:space:]' < "$ROOT/VERSION")" ]]
[[ ! -n "$(gofmt -l "$ROOT/backend/cmd/citizen-launcher")" ]]
python3 - <<'PY' "$ROOT"
import pathlib, sys, yaml
root=pathlib.Path(sys.argv[1])
for p in [root/'.gitea/workflows/ci.yml', root/'.gitea/workflows/release.yml']:
    yaml.safe_load(p.read_text())
assert not (root/'.github/workflows').exists(), 'legacy GitHub workflows must not shadow Gitea workflows'
release=(root/'.gitea/workflows/release.yml').read_text()
assert 'gh release' not in release
assert 'gitea.api_url' in release and 'gitea.token' in release
assert 'actions/upload-artifact' not in release and 'actions/download-artifact' not in release

import re
for wf in [root/'.gitea/workflows/ci.yml', root/'.gitea/workflows/release.yml']:
    text=wf.read_text()
    bad=re.findall(r'(?m)^\s*(?:run:\s*)?(\./[^\s]+\.sh)(?:\s|$)', text)
    assert not bad, f'workflow executes shell script directly: {wf}: {bad}'
print('Gitea workflow yaml: OK')
PY
echo 'Citizen Launcher verification: OK'
