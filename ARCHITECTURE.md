# Citizen Launcher architecture

## One source of truth

The Go core under `backend/cmd/citizen-launcher` owns detection, setup, repair, launch, maintenance, support and self-update. Shell/QML integrations are adapters only; they must not duplicate the gaming-stack implementation.

```text
Standalone GUI / Omarchy adapter
              │
              ▼
       Citizen Launcher Go core
              │
   ┌──────────┼───────────┐
   ▼          ▼           ▼
Preflight   Wine       RSI metadata
            selector       │
            + test          ▼
   │          │       verified installer
   └────┬─────┘             │
        ▼                   ▼
        managed Wine prefix
        │       │        │
        ▼       ▼        ▼
   Winetricks  DXVK  PowerShell wrapper
        └───────┬────────┘
                ▼
          RSI Launcher / Game
```

## Activation model

Mutable downloads are staged first. Executable GitHub release assets require a SHA-256 digest. A Wine candidate is extracted to a staging directory, tested against a throw-away prefix, and only then becomes `vendor/wine/current`. The previous validated runner is retained.

The real game prefix is never used as the Wine compatibility test target.

## Concurrency model

- `gui.lock`: one GUI backend process per user.
- in-memory GUI job gate: one long-running GUI action at a time.
- `maintenance.lock`: cross-process serialization of setup/repair/maintenance/launch transition.
- `/proc` exact `WINEPREFIX` inspection: maintenance is deferred while user-facing prefix processes are active.
- launch path rechecks RSI/Star Citizen after acquiring the stack lock, closing the double-click race.

## GUI

The UI is embedded in the static binary. It binds an ephemeral port on `127.0.0.1` and uses a cryptographically random per-process route token. No remote web content is needed. A second `citizen-launcher gui` discovers the locked instance and reopens that URL instead of spawning another backend.

## Distribution boundary

Citizen Launcher manages Wine/DXVK/RSI in XDG user directories. It does not replace distribution GPU drivers, kernels or package update policy. Debian packaging supplies only the system limits and a narrowly scoped package self-update timer; interactive system preparation uses Polkit rather than passwordless sudo.
