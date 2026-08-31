# Citizen Launcher 1.0.0 – Verification Report

Date: 2026-08-31

## Release gate

The 1.0.0 source tree passed the complete automated release gate:

- shell syntax checks for installer, uninstaller, packaging and integration scripts
- `gofmt` cleanliness
- `go test ./...`
- `go vet ./...`
- static Linux amd64 build
- `go test -race ./...`
- Omarchy updater integration test against disposable local Git repositories
- Debian package build and metadata/payload verification
- generic Linux amd64 tarball build and execution check
- process-level GUI single-instance test: a second GUI invocation reused the first localhost instance
- live GUI `/api/ping` check reporting version 1.0.0
- systemd service/timer syntax verification with `systemd-analyze verify`
- desktop entry newline regression check (no literal `\\n` in Exec entries)
- AppStream XML parse check
- GitHub Actions CI/release YAML parse check

## Regression coverage added for 1.0.0

The Go test suite explicitly covers:

- safe archive extraction and traversal/symlink rejection
- exact Wine-prefix process matching
- GUI and stack-operation file locks
- duplicate primary-action handler regression
- Star Citizen/RSI duplicate launch handling
- DXVK DLL/override state
- deterministic Winetricks checksum and minimal base verbs
- legacy user-local binary migration
- broken desktop entry repair from the 0.9.2 literal-newline bug
- copied executable permission preservation
- Debian update package name/version/architecture validation
- release asset selection/version comparison
- Electron-builder RSI `latest.yml` variants
- non-MSI portable PowerShell compatibility path
- Wine registry path escaping
- support-bundle sanitizer behavior
- required SHA-256 release digest fail-closed behavior

## Final binary / package hashes

- `backend/bin/citizen-launcher`: `5e55630dc47b89a11185384f09d79fb3813fce51b4d0e30a9660e6526c93a46d`
- `dist/citizen-launcher_1.0.0_amd64.deb`: `976a4cb3efc193f9f05e880ab0456d80e49cb7b5db0a61621d4dc4a788e7c8a1`
- `dist/citizen-launcher-1.0.0-linux-amd64.tar.gz`: `551bca5656382f79fd6e388d14f58555d0db66418a0388ab8a8ba2ac1a058e90`

## Scope boundary

The release gate validates the launcher, package, updater, GUI, locks, migration, install/repair logic and local safety properties in the build environment. It cannot perform an RSI account login, download the full Star Citizen game, or execute a real game session with an external GPU from this sandbox. Those remain real-machine acceptance tests rather than simulated release-gate claims.
