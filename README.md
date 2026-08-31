# Citizen Launcher 0.9.1

Citizen Launcher is the distro-neutral successor to Omarchy Citizen.

## Goal

**Install Linux → install Citizen Launcher → log into RSI → play.**

The core is one static Go binary and does not depend on Omarchy. Omarchy is an optional integration under `integrations/omarchy/`.

## Supported targets

Primary targets:

- Debian / Ubuntu / Linux Mint
- Fedora
- Arch Linux / Omarchy
- openSUSE (generic user install)

Other amd64 Linux distributions can use the generic tarball when they provide a working Vulkan driver and the runtime requirements of the selected Wine runner.

## GUI

`citizen-launcher gui` starts a localhost-only UI from files embedded in the Go binary. Chromium-family browsers open it as a dedicated app window; otherwise it opens in the default browser. No GTK/Qt/WebKit development/runtime package is required by Citizen Launcher itself.

## Architecture

```text
Standalone GUI / Omarchy bar integration
                 ↓
           Citizen Launcher
             Go core
                 ↓
 ┌───────────────┼────────────────┐
 Hardware      Wine             RSI Launcher
 readiness     selector          installer
               + self-test
                 ↓
             Wine prefix
                 ↓
       Winetricks + DXVK
                 ↓
          Star Citizen
```

The managed game stack lives in XDG user directories:

- `~/.config/citizen-launcher`
- `~/.local/share/citizen-launcher`
- `~/.local/state/citizen-launcher`
- `~/.cache/citizen-launcher`

Existing `omarchy-citizen` data is migrated when possible.

## Build

```bash
./build.sh
```

## Install (any desktop distro)

```bash
./install.sh
```

For Omarchy plus bar integration:

```bash
./install-omarchy.sh
```

## Debian package

```bash
./packaging/build-deb.sh
```

## Security boundaries

Citizen Launcher manages its Wine/DXVK/RSI stack entirely as the current user. It does not create passwordless sudo/pacman/apt rules. GPU/kernel/base-system updates remain owned by the distribution.


## 0.9.1 fixes

- PowerShell is no longer a mandatory prefix component.
- RSI `latest.yml` parser accepts Electron Builder `path:` and nested `files: - url:` formats.
- Repair also refreshes DXVK.
- Responsive dashboard prevents long GPU/distribution names from overflowing.
- GUI shows concise user-facing errors with expandable technical details.
