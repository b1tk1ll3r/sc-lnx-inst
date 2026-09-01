# Citizen Launcher architecture

## One gaming core, multiple Linux integrations

The Go core under `backend/cmd/citizen-launcher` is the only implementation of detection, setup, repair, launch, maintenance, support and self-update. Debian/RPM/Arch packages and the optional Omarchy widget are adapters around that core.

```text
          Standalone GUI / Omarchy adapter
                     │
                     ▼
             Citizen Launcher Go core
                     │
       ┌─────────────┼──────────────┐
       ▼             ▼              ▼
 Distro/preflight   Wine        RSI metadata
 + package mode     selector      + verifier
       │             + test          │
       └──────────┬──┴───────┬───────┘
                  ▼          ▼
              managed Wine prefix
             Winetricks / DXVK /
             PowerShell wrapper
                  │
                  ▼
           RSI Launcher / Game
```

## Distribution abstraction

`platform.go` normalizes `/etc/os-release` into Debian, Fedora/RHEL, Arch, SUSE or generic families. Package-manager discovery is family-aware so a foreign tool in `$PATH` cannot accidentally select the wrong update strategy.

Native package formats share the same payload:

- `/usr/bin/citizen-launcher`
- freedesktop desktop entry, icon and AppStream metadata
- `vm.max_map_count` and file-limit policy
- systemd package self-update timer
- release-repository trust configuration

Only packaging metadata and the native package database differ.

## Mutable vs immutable systems

Mutable package installs may update Citizen Launcher through APT, RPM or pacman after release-digest and package-metadata verification. OSTree/transactional/SteamOS-style systems are detected and their base image is left untouched. A user-mode install uses the verified generic tarball instead.

## Activation and rollback model

Mutable gaming-stack downloads are staged first. Executable release assets require an expected digest. A Wine candidate is extracted to a staging directory, tested against a throw-away prefix, and only then becomes `vendor/wine/current`; the previous validated runner is retained.

The real game prefix is never used as the Wine compatibility test target.

## Concurrency model

- `gui.lock`: one GUI backend process per user.
- in-memory GUI job gate: one long-running GUI action at a time.
- `maintenance.lock`: cross-process serialization of setup/repair/maintenance/launch transitions.
- exact `/proc` `WINEPREFIX` inspection: maintenance is deferred while RSI/Star Citizen/Wine tools use the prefix.
- launch path rechecks RSI/Star Citizen after taking the stack lock.

## Security boundary

Citizen Launcher never creates passwordless sudo rules. Interactive privileged preparation uses Polkit/pkexec. Base-system upgrades, GPU drivers, kernel and firmware remain under the distribution's own update mechanism.
