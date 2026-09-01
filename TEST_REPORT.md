# Citizen Launcher 1.1.3 – Verification Report

Date: 2026-09-01

## Release scope

Citizen Launcher 1.1.3 keeps the confirmed playable multi-distribution 1.1.x gaming core and hardens Gitea-native RPM CI against constrained Fedora job-container storage.

## Failure reproduced from real Gitea runner log

The real Fedora 44 job reached dependency installation but failed before checkout/build because the transaction could not fit on `/`. The original dependency set expanded to hundreds of packages. The log ultimately reported that packages needed roughly 407 MB more root filesystem space and the `Build dependencies` step exited with status 1.

1.1.3 therefore treats CI container footprint as a release property, not a runner-specific workaround.

## Automated release gate

The 1.1.3 tree passes the local release gate with:

- shell syntax checks for installers, packaging, Gitea release helper and tests
- `gofmt` cleanliness
- `go test ./...`
- `go vet ./...`
- static Linux amd64 build and version check
- `go test -race ./...`
- Omarchy updater integration test against disposable local Git repositories
- Debian package build and metadata/payload verification
- generic Linux amd64 tarball build and execution check
- Gitea workflow YAML parsing
- assertion that legacy `.github/workflows/` does not shadow `.gitea/workflows/`
- assertion that release publishing does not use `gh release` or cross-job artifact actions
- mocked Gitea REST API test covering release creation, update on rerun, asset upload, replacement and download
- Gitea release helper exercised with both Python and `jq` JSON backends
- Gitea release-source parsing and `SHA256SUMS.txt` parser regression tests
- static validation of RPM spec and Arch PKGBUILD/package hooks
- multi-distro platform-family and immutable-host regression tests
- native package self-update parser/asset-selection tests for DEB, RPM and pacman formats
- mode-independent shell invocation checks for Gitea/act checkouts without execute bits
- low-disk Fedora RPM workflow policy regression test

## Fedora RPM low-disk policy

Both `.gitea/workflows/ci.yml` and `.gitea/workflows/release.yml` now enforce this shape:

- DNF cache moved to the mounted Gitea workspace via `system_cachedir`
- `install_weak_deps=False`
- `tsflags=nodocs`
- `git-core`, not the full `git` package
- `golang-bin`, not the `golang` meta package
- `golang-bin` removed before `rpm-build` is installed
- no explicit `systemd-rpm-macros` in the build container
- no Python requirement for Fedora release upload; `jq` is used by `gitea-release.sh`

`tests/rpm-ci-footprint.sh` fails if the heavy one-shot dependency pattern is reintroduced.

## Gitea CI/CD design verified

Workflows live in:

- `.gitea/workflows/ci.yml`
- `.gitea/workflows/release.yml`

Release packages are uploaded directly to the Gitea Release. The final job downloads the native packages and publishes a deterministic `SHA256SUMS.txt`. The helper is rerun-safe and replaces same-named attachments.

## Native package build coverage

The local verification environment builds and inspects:

- `dist/citizen-launcher_1.1.3_amd64.deb`
- `dist/citizen-launcher-1.1.3-linux-amd64.tar.gz`

The Gitea workflows build these in native job containers:

- `citizen-launcher-1.1.3-1.linux.x86_64.rpm`
- `citizen-launcher-1.1.3-1-x86_64.pkg.tar.zst`

The current sandbox does not provide Docker/rpmbuild, so the exact Fedora container transaction cannot be executed locally. Instead, the workflow structure is regression-tested and the full application/package suite is executed locally. The next Gitea Fedora run is the authoritative integration check for the runner's actual storage quota.

## Gaming-core acceptance

The Star Citizen install/play path is unchanged from the already accepted 1.0.1/1.1.x line. This release changes CI/release plumbing, not Wine/DXVK/RSI launch behavior.
