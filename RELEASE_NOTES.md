# Citizen Launcher 0.9.2

Maintenance/UX hotfix for the distro-neutral preview.

- Removed mandatory PowerShell Core from base Winetricks setup.
- Robust RSI Electron Builder `latest.yml` parsing (`path`, nested `url`, version fallback).
- DXVK is included in automatic repair.
- Responsive GUI with safer grid sizing and readable diagnostics.
- Added regression tests for RSI metadata formats and the minimal prefix contract.

## 0.9.2

- fixes `Wine Registry: reg: Angegebener Schlüssel nicht zugreifbar oder erstellbar`
  - registry path now uses valid single separators
  - file-association tweak is non-fatal because it is not required by Star Citizen
- adds automatic application updates
  - Debian package installs a root systemd updater timer
  - updater checks GitHub Releases every six hours
  - downloads only the expected `citizen-launcher_<version>_amd64.deb`
  - requires and verifies GitHub's SHA-256 asset digest
  - verifies package name/version/architecture before APT installation
  - generic user installs self-update from the verified tarball
- GUI shows whether automatic package updates are active
- a running old GUI detects when a newer package was installed and offers a one-click restart
- adds a GitHub Actions release workflow and central `VERSION` file
