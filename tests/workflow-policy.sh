#!/usr/bin/env bash
# Static policy checks for GitHub Actions workflows and RPM packaging. Guards
# against regressions of problems that previously broke the RPM pipeline.
set -euo pipefail
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
python3 - "$ROOT" <<'PY'
from pathlib import Path
import re, sys, yaml
root = Path(sys.argv[1])
wf_dir = root/'.github/workflows'
assert not (root/'.gitea/workflows').exists(), '.gitea/workflows would shadow/duplicate GitHub workflows'
files = sorted(wf_dir.glob('*.yml'))
assert {p.name for p in files} >= {'ci.yml', 'release.yml', 'packages.yml'}, files
VALID_PERMS = {'actions','attestations','checks','contents','deployments','discussions','id-token',
               'issues','models','packages','pages','pull-requests','repository-projects',
               'security-events','statuses'}
for p in files:
    text = p.read_text()
    data = yaml.safe_load(text)
    name = p.name
    assert 'gitea.' not in text, f'{name}: Gitea-only context used'
    assert data.get('permissions') == {'contents': 'read'}, f'{name}: top-level permissions must be contents: read'
    for job_id, job in data['jobs'].items():
        perms = job.get('permissions', {})
        unknown = set(perms) - VALID_PERMS
        assert not unknown, f'{name}:{job_id}: invalid permission scopes {unknown}'
        if any(v == 'write' for v in perms.values()):
            assert (name, job_id) == ('release.yml', 'publish'), f'{name}:{job_id}: write permission outside publish job'
        if 'container' in job:
            shell = (job.get('defaults') or data.get('defaults') or {}).get('run', {}).get('shell')
            assert shell == 'bash', f'{name}:{job_id}: container job needs bash as default shell'
        for step in job.get('steps', []):
            uses = step.get('uses', '')
            if uses and not uses.startswith('./'):
                assert re.search(r'@(v\d+|[0-9a-f]{40})$', uses), f'{name}:{job_id}: unpinned action {uses}'
            if uses.startswith('actions/checkout'):
                assert (step.get('with') or {}).get('persist-credentials') is False, f'{name}:{job_id}: checkout must not persist credentials'
            run = str(step.get('run', ''))
            assert '${{ inputs.' not in run and '${{ github.event' not in run, f'{name}:{job_id}: expression injected into run script'
# Releases must be signed and ship the installer.
release = (root/'.github/workflows/release.yml').read_text()
for needle in ['release-sign sign', 'SHA256SUMS.txt', 'secrets.RELEASE_SIGNING_KEY', 'install.sh', 'openssl pkeyutl -verify']:
    assert needle in release, f'release.yml missing: {needle}'
# RPM packaging of a prebuilt static Go binary.
spec = (root/'packaging/rpm/citizen-launcher.spec').read_text()
for needle in ['%global debug_package %{nil}', '%global _build_id_links none']:
    assert needle in spec, f'RPM spec missing: {needle}'
build = (root/'backend/build.sh').read_text()
for needle in ['-B gobuildid', '-buildvcs=false', 'CGO_ENABLED=0']:
    assert needle in build, f'backend/build.sh missing: {needle}'
print('Workflow and packaging policy: OK')
PY
