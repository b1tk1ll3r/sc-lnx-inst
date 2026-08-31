# Omarchy Citizen 0.7.0

## Rundum-Sorglos / Autopilot

Omarchy Citizen 0.7.0 changes the architecture from "frontend for packages the
user maintains" to "self-maintaining Star Citizen appliance".

Autopilot manages the complete user-space gaming stack:

- Omarchy Citizen plugin updates (for clean Git-managed installs)
- official Star Citizen LUG Helper **AppImage**
- stable `lug-wine-tkg-git` Wine runner
- DXVK
- Wine prefix health testing
- launch-script Wine runner selection
- repair / recovery
- support logs

The LUG Helper AppImage is preferred over the AUR package. Current LUG releases
bundle their GUI/runtime dependencies, reducing the amount of Arch package
knowledge required from the player.

### Update model

A systemd user timer runs every six hours. Each candidate Wine runner is tested
in a throw-away Wine prefix before it can become the active runner. The test
requires:

- successful `wineboot`
- `drive_c`
- `system.reg`
- `user.reg`
- a valid `%APPDATA%` response

If an upstream runner fails that test, the previous working runner stays active.

DXVK is only changed while the Star Citizen Wine prefix is not in use. Existing
DLLs are backed up before replacement.

### What Autopilot deliberately does not bypass

Kernel, GPU driver and base Omarchy package upgrades still belong to Omarchy's
blessed `omarchy update` pipeline. Omarchy Citizen does not install a passwordless
pacman/sudo backdoor.

That boundary lets the Star Citizen stack remain self-maintaining without giving
a third-party QML plugin permanent unattended root package-management rights.

## Local state

Managed components:

```text
~/.local/share/omarchy-citizen/vendor/
├── lug-helper/
├── wine/
└── dxvk/
```

Logs:

```text
~/.local/state/omarchy-citizen/
├── omarchy-citizen.log
└── updater.log
```

## License

MIT.
