#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"

python3 - "$ROOT" <<'PY'
from pathlib import Path
import re, sys
root = Path(sys.argv[1])
check = [
    root/'build.sh', root/'install.sh', root/'integrations/omarchy/install.sh',
    root/'tests/installer-test.sh', root/'tests/smoke-citizenctl.sh',
    root/'tests/full-verify.sh', root/'tests/verify.sh', root/'tests/package-verify.sh',
    root/'tests/workflow-policy.sh', root/'packaging/build-all.sh',
    *sorted((root/'.github/workflows').glob('*.yml')),
]
for p in check:
    text = p.read_text()
    bad = []
    for lineno, line in enumerate(text.splitlines(), 1):
        stripped = line.strip()
        if stripped.startswith('#') or stripped.endswith('\\') or stripped.endswith('; do'):
            continue
        # Direct ./script.sh command (workflow or shell command) without an interpreter.
        if re.search(r'(^|[;&|]\s*|run:\s*)\./[^\s"\x27]+\.sh(?:\s|$)', stripped):
            bad.append((lineno, stripped))
        # Direct "$ROOT/path/script.sh" command at start of a shell line.
        if re.search(r'^"?\$ROOT/[^"\s]+\.sh"?(?:\s|$)', stripped):
            bad.append((lineno, stripped))
    if bad:
        raise SystemExit(f'direct shell execution remains in {p}: {bad}')
print('Mode-independent shell invocation: OK')
PY
