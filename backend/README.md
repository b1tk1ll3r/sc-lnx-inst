# Citizen Launcher Go Core

The Go core is the single source of truth for Star Citizen setup, launch, repair and maintenance.

## Commands

```text
citizen-launcher gui
citizen-launcher platform [--json]
citizen-launcher game-status [--json]
citizen-launcher game-install
citizen-launcher game-launch
citizen-launcher game-repair
citizen-launcher maintain
citizen-launcher doctor
citizen-launcher support
citizen-launcher autopilot enable|disable|status
```

The optional Omarchy integration also exposes the legacy integration update commands.

Build with `./build.sh`. The output is `bin/citizen-launcher`.
