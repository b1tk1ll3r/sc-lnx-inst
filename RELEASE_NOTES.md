# Citizen Launcher 1.1.3

## Gitea Fedora/RPM runner hardening

1.1.3 keeps the confirmed playable multi-distribution 1.1.x runtime and fixes the next real Gitea `act_runner` failure found in the Fedora RPM job.

### Root cause

The previous workflow installed `git`, the full `golang` meta package, `rpm-build`, `systemd-rpm-macros`, `curl` and `python3` in one Fedora transaction. Fedora also enabled weak dependencies by default. On a small Docker root filesystem this expanded into hundreds of packages and exhausted `/` before the RPM build even started.

### Fix

- Fedora jobs now use `git-core` instead of the full Git package.
- Go builds use `golang-bin` rather than the `golang` meta package, avoiding the large source package.
- `systemd-rpm-macros`, Python and other unnecessary RPM-build dependencies are no longer installed in the Fedora build phase.
- DNF weak dependencies are disabled and documentation payloads are skipped in CI containers.
- DNF's root cache is redirected to the mounted Gitea workspace volume.
- Downloaded RPM cache files are deleted after each transaction.
- The static Go backend is built first, then `golang-bin` is removed before `rpm-build` is installed. The two heavy toolchains no longer occupy the container root simultaneously.
- The Gitea release helper now supports a `jq` JSON backend, so the Fedora release job does not need the full Python runtime just to upload an RPM.
- Disk usage is printed between phases to make future runner-capacity problems immediately visible.

### Regression guards

- `tests/rpm-ci-footprint.sh` asserts the low-disk policy in both CI and release RPM jobs.
- Gitea release-helper tests now exercise both Python and `jq` JSON backends.
- The earlier `0644`/`Permission denied` workflow hardening remains in place.

### Runtime

Wine, DXVK, RSI Launcher setup, hardware checks, single-instance protection, repair, support bundles and the already confirmed playable Star Citizen path are unchanged.
