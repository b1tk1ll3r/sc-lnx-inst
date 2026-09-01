# Citizen Launcher 1.1.2 – Verification Report

Date: 2026-09-01

## Release scope

Citizen Launcher 1.1.2 keeps the confirmed playable 1.1.x multi-distribution gaming core and hardens the Gitea-native CI/CD introduced in 1.1.1 against checkout permission differences.

## Automated release gate

The 1.1.2 tree passes the local release gate with:

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
- assertion that the release workflow does not use `gh release` or cross-job artifact actions
- mocked Gitea REST API test covering release creation, release update on rerun, asset upload, same-name asset replacement and asset download
- Gitea release-source parsing and `SHA256SUMS.txt` parser regression tests
- static validation of RPM spec and Arch PKGBUILD/package hooks
- multi-distro platform-family and immutable-host regression tests
- native package self-update parser/asset-selection tests for DEB, RPM and pacman formats
- full regression suite repeated successfully after forcing every repository `*.sh` file to mode `0644`; CI does not depend on executable bits
- regression guard rejects direct repository `.sh` execution in Gitea workflow entry points

## Gitea CI/CD design verified

Workflows live in:

- `.gitea/workflows/ci.yml`
- `.gitea/workflows/release.yml`

The release workflow uses Gitea-native contexts (`gitea.api_url`, `gitea.repository`, `gitea.ref_name`, `gitea.sha`, `gitea.token`) and the built-in job token with `code: read` / `releases: write` permissions.

Release packages are uploaded directly to the Gitea Release rather than moved between jobs with `actions/upload-artifact`. The final job downloads the native release packages and publishes one deterministic `SHA256SUMS.txt`.

The release helper is rerun-safe: an existing release is refreshed and same-named attachments are replaced instead of duplicated.

## Gitea-aware launcher self-update

A Gitea workflow build injects this form as the launcher's trusted update source:

```text
gitea:<gitea.api_url>/repos/<owner>/<repo>
```

The launcher resolves Gitea's latest release API and maps the release `SHA256SUMS.txt` back onto the package assets before a privileged update is allowed. Privileged Gitea sources require HTTPS. Existing GitHub release-source syntax remains supported for migration/backward compatibility.

A separate build check confirmed that a `gitea:https://.../api/v1/repos/owner/repo` source is successfully embedded into the static binary and reported by `self-update status`.

## Native package build coverage

The current local build environment contains Debian packaging tools, therefore these artifacts can be built and inspected locally:

- `dist/citizen-launcher_1.1.2_amd64.deb`
- `dist/citizen-launcher-1.1.2-linux-amd64.tar.gz`

The local environment does not provide native `rpmbuild` / Arch `makepkg`; the Gitea workflows build those in Fedora and Arch job containers:

- `citizen-launcher-1.1.2-1.linux.x86_64.rpm`
- `citizen-launcher-1.1.2-1-x86_64.pkg.tar.zst`

## Gaming-core acceptance

The Star Citizen install/play path is unchanged from the already accepted 1.0.1/1.1.0 line. The CI migration changes release and update plumbing, not Wine/DXVK/RSI launch behavior.

## Checkout permission regression

The complete `tests/full-verify.sh` suite was repeated after forcing every repository `*.sh` file to Unix mode `0644`. It still completed successfully. This reproduces the Gitea/act checkout condition that caused exit code 126 and verifies that CI/CD no longer depends on executable bits for repository shell scripts. Distributed archives still mark shell scripts executable for convenience.
