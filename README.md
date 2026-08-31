# Omarchy Citizen 0.8.0 — Development Preview

Omarchy Citizen is moving from a frontend around the LUG installer to an
**owned, self-maintaining Star Citizen stack for Omarchy**.

## Architecture

The critical install and launch path is owned by this project:

1. hardware / Vulkan readiness gate
2. select the newest locally compatible stable LUG Wine runner
3. validate it in a disposable Wine prefix
4. create the real Wine prefix
5. dynamically fetch current Winetricks and install required verbs
6. dynamically fetch and verify the current RSI Launcher
7. manage DXVK and the stable launch entry point
8. periodically maintain the user-space gaming stack

LUG Helper is **optional support/compatibility tooling**. Its failure must not
block installation and it never chooses the active Wine runner.

## Stable paths

User-space components live below:

```text
~/.local/share/omarchy-citizen/
~/.local/state/omarchy-citizen/
~/.config/omarchy-citizen/
```

The backend intentionally hides upstream version/layout changes behind stable
`vendor/*/current` paths and its own game configuration.

## Update safety

- Wine candidates are tested before activation.
- Incompatible immutable Wine releases are cached per CPU/glibc fingerprint.
- Partial prefixes are backed up rather than deleted.
- RSI installer downloads are checked against the SHA-512 value from its
  published update metadata.
- DXVK is not changed while the Wine prefix is active.
- Plugin Git updates retain the Omarchy validation / trusted-remote protection.

## Current development status

See `DEVELOPMENT_STATUS.md`.

## License

MIT.
