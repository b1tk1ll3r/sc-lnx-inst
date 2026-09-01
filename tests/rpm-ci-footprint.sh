#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
python3 - "$ROOT" <<'PY'
from pathlib import Path
import re, sys, yaml
root = Path(sys.argv[1])
checks = [
    (root/'.gitea/workflows/ci.yml', 'rpm-package'),
    (root/'.gitea/workflows/release.yml', 'rpm'),
]
for path, job_name in checks:
    data = yaml.safe_load(path.read_text())
    job = data['jobs'][job_name]
    runs = '\n'.join(str(step.get('run','')) for step in job.get('steps',[]))
    flat = re.sub(r'\\\n\s*', ' ', runs)
    flat = re.sub(r'\s+', ' ', flat)
    required = [
        'system_cachedir=',
        'install_weak_deps=False',
        'tsflags=nodocs',
        'install git-core',
        'install golang-bin',
        'remove golang-bin',
        'install rpm-build',
    ]
    for needle in required:
        if needle not in flat:
            raise SystemExit(f'{path.name}:{job_name} missing low-disk guard: {needle}')
    forbidden = ['systemd-rpm-macros', 'install git golang', 'install golang rpm-build', 'install python3']
    for needle in forbidden:
        if needle in flat:
            raise SystemExit(f'{path.name}:{job_name} reintroduced heavy dependency set: {needle}')
    if flat.index('install golang-bin') > flat.index('remove golang-bin') or flat.index('remove golang-bin') > flat.index('install rpm-build'):
        raise SystemExit(f'{path.name}:{job_name} no longer stages Go before rpmbuild')
print('RPM CI low-disk dependency policy: OK')
PY
