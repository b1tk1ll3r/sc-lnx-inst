# Citizen Launcher 1.0.0

First productized distro-neutral release.

## Setup and gameplay

- Replaces the old LUG-controlled installation path with one Citizen Launcher core.
- Chooses the newest stable LUG Wine release that actually passes a fresh-prefix self-test on the local CPU/glibc environment; incompatible releases are cached and skipped.
- Preserves a previous working Wine runner and never activates an untested candidate.
- Uses a deterministic SHA-256-pinned Winetricks 20260125 base (`arial`, `tahoma`, `win11` only).
- Avoids the unreliable Winetricks PowerShell MSI path. RSI PowerShell compatibility is provided by verified portable PowerShell Core 7.4.19 and the RSI-compatible Wine wrapper 3.0.5.
- Manages DXVK itself, including DLL verification and Wine native overrides.
- Parses current Electron Builder RSI `latest.yml` layouts and verifies the RSI installer SHA-512 before execution.
- Repairs prefix components, RSI compatibility, DXVK, RSI Launcher and desktop integration without deleting game data.

## Reliability hardening

- Fixes the historical literal `\\n` Star Citizen desktop-entry regression and automatically repairs old affected launchers.
- Fixes Wine registry path escaping; the file-association tweak is non-fatal.
- Fixes executable permissions when self-sync copies over an existing temporary file.
- Single GUI instance via file lock; a second launch reuses the existing localhost UI.
- One active GUI job at a time plus a cross-process gaming-stack lock.
- Detects a running RSI Launcher or Star Citizen and does not launch duplicates or kill an active game.
- Exact `/proc` `WINEPREFIX` matching avoids false busy/running detections for similarly named prefixes.
- Wine and DXVK release archives use safe extraction with traversal/link checks.
- Downloads are staged and atomically activated; logs rotate and maintenance is deferred while the prefix is busy.

## System and UX

- Responsive standalone GUI with concise errors and expandable technical diagnostics.
- Preflight for Vulkan, AVX, RAM+swap, free storage, Linux filesystem, `vm.max_map_count` and open-file limits.
- Debian package installs the recommended system limits and uses Polkit for interactive preparation when needed.
- User-local 0.9.x binaries/desktop entries that shadow a newer Debian package are migrated safely.
- Generic uninstall removes launcher integration but preserves game/config data.

## Updates and maintenance

- Debian package installs a systemd timer that checks for a new Citizen Launcher release about every six hours.
- Automatic package update requires the expected GitHub release asset, a SHA-256 digest, matching `citizen-launcher` package name/version and `amd64` architecture before APT is invoked.
- Root updater ignores user-controlled release-repository environment variables and rejects writable/non-root repository configuration.
- Running GUIs are not killed during package replacement; they offer a controlled restart into the installed version.
- CI/release gate runs unit tests, vet, race detector, static build, integration test and package verification.
