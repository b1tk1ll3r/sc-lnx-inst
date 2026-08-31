# Architecture

## Layers

1. **Core (`backend/cmd/citizen-launcher`)** – single source of truth for detection, setup, repair, launch and maintenance.
2. **Standalone GUI** – embedded HTML/CSS/JS served only on `127.0.0.1` behind a random per-process URL token.
3. **Desktop integration** – `.desktop` launcher, systemd user maintenance timer when available.
4. **Distribution adapters** – `/etc/os-release` and package-manager detection. The core does not depend on a package manager for Wine/DXVK/RSI.
5. **Omarchy adapter** – optional Quickshell plugin that calls the same Go core.

## Why a local embedded web GUI?

It lets one static Go binary present the same polished UI on Debian, Fedora and Arch without linking to a particular desktop toolkit. The HTTP server binds to loopback only and uses an unguessable route token. No remote content is required to render the UI.

## Update strategy

Gaming components are versioned independently and activated only after local validation. Wine uses newest-compatible fallback rather than newest-at-all-costs. Distribution packages and GPU drivers are outside the self-update trust boundary.
