# Citizen Launcher 1.0.1 – Verification Report

Date: 2026-08-31

## Release gate

The 1.0.1 source tree passed the complete automated release gate:

- shell syntax checks for installer, uninstaller, packaging and integration scripts
- `gofmt` cleanliness
- `go test ./...`
- `go vet ./...`
- static Linux amd64 build
- `go test -race ./...`
- Omarchy updater integration test against disposable local Git repositories
- Debian package build and metadata/payload verification
- generic Linux amd64 tarball build and execution check
- systemd service/timer syntax verification
- desktop entry regression checks
- AppStream and workflow validation inherited from the 1.0 release gate

## Real-machine regressions reproduced from the support bundle

The uploaded Debian 13 support bundle exposed two 1.0.0 false negatives:

1. `vulkaninfo` reported both `AMD Radeon Graphics (RADV PHOENIX2)` and Mesa `llvmpipe`. 1.0.0 rejected the whole system merely because the software ICD was present. 1.0.1 parses devices independently and selects the real AMD GPU.
2. The RSI PowerShell wrapper returned a successful process exit but no captured stdout. 1.0.0 incorrectly required a marker string in stdout. 1.0.1 validates PowerShell Core and then treats the wrapper's propagated exit status as authoritative.

Automated regression tests cover both conditions.

## Additional process-level verification

A real two-process GUI test was run against the built binary:

- first `gui --no-open` created a localhost endpoint
- second `gui --no-open` returned the exact same endpoint instead of creating another backend
- `/api/ping` on that endpoint reported version `1.0.1`

A synthetic `vulkaninfo` run matching the support bundle's AMD + llvmpipe layout selected `AMD Radeon Graphics (RADV PHOENIX2)` and reported Vulkan `ready`. The sandbox itself has insufficient RAM for Star Citizen, so its overall hardware state remained blocked for RAM as expected; the GPU path was no longer the blocker.

## Final binary / package hashes

- `backend/bin/citizen-launcher`: `7cd0d85046b2c9ac295c6ef5b86f93a3452bfb3cd94cff048e7511db581f3d07`
- `dist/citizen-launcher_1.0.1_amd64.deb`: `aba0944a0d2334f14f425e12a55ddd28d011449387e5be1ef3e0731e9d9a5119`
- `dist/citizen-launcher-1.0.1-linux-amd64.tar.gz`: `59edf81938c305b1d86d917d38222470b4f5e42c9c3dc350bf1ac430d8748bfc`

## Scope boundary

The release gate validates the launcher, package, updater, GUI, locks, migration, install/repair logic and local safety properties. It cannot perform an RSI account login or a complete live Star Citizen game session from this sandbox. Those remain real-machine acceptance tests.
