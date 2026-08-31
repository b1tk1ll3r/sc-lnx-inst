# Omarchy Citizen 0.6.2

A plug-and-play Star Citizen control panel for Omarchy.

## Design goal

**Play Star Citizen, don't administer Linux.**

The default UI deliberately exposes only:

- Setup / Start
- Guided repair
- Create support bundle

Advanced Wine/DXVK/LUG/NGL controls remain available behind an advanced section.

## 0.6.0 highlights

- one primary action that adapts to current system state
- guided first-time setup
- symptom-based repair assistant
- persistent plugin debug log
- one-click sanitized support bundle
- support ID for remote troubleshooting
- Omarchy's native `omarchy debug --no-sudo --print` included in support bundles
- advanced controls hidden by default
- optional NGL remains isolated from the normal launcher path
- Go-based plugin update backend with Git remote pinning
- optional automatic updates via systemd user timer
- updater status/log included in support bundles

## Local debug log

```text
~/.local/state/omarchy-citizen/omarchy-citizen.log
```

The log rotates automatically at roughly 4 MiB and keeps the newest ~2 MiB.

## Support bundle

One click creates:

```text
~/Downloads/Omarchy-Citizen-Support-<support-id>.tar.gz
```

The exported data is sanitized on a best-effort basis before packaging.

See [`INSTALLATION.md`](INSTALLATION.md) for details.

## License

MIT.

## Go update backend

The bundled static Go binary lives at:

```text
backend/bin/omarchy-citizen-backend
```

For Git-managed Omarchy installations it checks the checkout and delegates actual
updates to Omarchy's native `omarchy plugin update local.omarchy-citizen --yes`.
Automatic updates are opt-in and run through a systemd user timer roughly every six
hours. The Git `origin` present when auto-update is enabled is pinned as the trusted
remote; unattended updates stop if that remote changes or the checkout is dirty.

See [`backend/README.md`](backend/README.md) and [`GIT_DISTRIBUTION.md`](GIT_DISTRIBUTION.md).


## 0.6.2 hotfix

- primary/setup actions now run through a Quickshell `Process` instead of fire-and-forget `bar.run`
- the panel shows `WIRD GESTARTET` immediately after a click
- startup failures stay visible in the panel instead of silently closing it
- terminal launch has a monitored fallback path and logs immediate launcher failures
- installers restart the Omarchy shell after QML changes to avoid stale third-party plugin QML

## 0.6.2 UI / setup hotfix

- fixed `undefined` in reusable status rows and chips
- panel no longer inherits the bar's theme font; uses a neutral system sans font
- first-time setup keeps the panel open and shows a visible status message
- setup terminal is launched directly through `xdg-terminal-exec`/UWSM with separate argv
- failed package/AUR setup remains visible instead of the terminal disappearing immediately
