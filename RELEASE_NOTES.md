# Unreleased (next release)

> Rename this heading to the new version and bump `VERSION` before tagging; this file is the GitHub release body.

## CI/CD moved to GitHub Actions

- `.gitea/workflows` replaced by `.github/workflows/{ci,release,packages}.yml`. CI and release share one reusable pipeline.
- Backend built once; `.deb`/tarball, `.rpm` (Fedora 44) and `.pkg.tar.zst` (Arch) packaged from the same binary and install-tested on Debian 13, Ubuntu 24.04, Fedora 44, openSUSE Tumbleweed and Arch.
- Releases are published with `gh`, including `SHA256SUMS.txt` and build-provenance attestations; least-privilege workflow permissions.

## RPM build fixes

- Spec disables debuginfo/build-id link generation for the prebuilt static Go binary (`debug_package %{nil}`, `_build_id_links none`); backend links with `-B gobuildid` and `-buildvcs=false`.
- `%config(noreplace)` for files under `/etc`, `%dir` ownership of `/etc/citizen-launcher`; duplicate spec removed.

## Signed releases and new installer

- `SHA256SUMS.txt` is signed with an Ed25519 release key (`SHA256SUMS.txt.sig`). The self-updater refuses unsigned or mis-signed releases and takes asset digests only from the signed manifest.
- New `install.sh` (published with every release): `bash <(wget -qO- https://github.com/b1tk1ll3r/sc-lnx-inst/releases/latest/download/install.sh)`. It verifies the signature with OpenSSL, installs via apt/dnf/zypper/pacman or into `~/.local`, and supports `--user`, `--version`, `--uninstall`, `--local`.
- Releases are install-tested end-to-end with that one-liner on Debian, Ubuntu, Fedora, openSUSE and Arch.
- GUI login via a single-use link exchanged for an `HttpOnly`/`SameSite=Strict` session cookie; no long-lived secret in URLs or process lists; CSP without `unsafe-inline`.
- Repository cleanup: removed committed binaries (`dist/`, `backend/bin/`), legacy Omarchy plugin files in the repo root, the unused `omarchy-citizen-backend`, `INSTALLIEREN.sh`/`uninstall.sh` (replaced by `install.sh`) and outdated 0.8.x documents. The Omarchy widget installs via `integrations/omarchy/install.sh`.

## Security hardening

- All downloads HTTPS-only (redirect downgrades refused) with a size cap; `verifyReleaseDigest` fails closed.
- RSI installer accepted only from `install.robertsspaceindustries.com` and only with a SHA-512.
- Self-update accepts only `X.Y.Z` tags and plain asset file names.
- Tar extraction rejects symlink chains that resolve outside the target.
- GUI: Host/Origin checks (DNS rebinding / CSRF), restart refused while a job runs.
- Support bundle created 0600, redaction extended to JSON keys and RSI identifiers.
- System updater service gets `HOME`/state directory (systemd sets no `$HOME`) and additional sandboxing.
- Default self-update source now `github:b1tk1ll3r/sc-lnx-inst`.

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
