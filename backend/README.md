# Omarchy Citizen Go Update Backend

`omarchy-citizen-backend` is a small, dependency-free Go helper for plugin updates.
It deliberately does **not** implement its own plugin package manager. For Git-managed
Omarchy plugins it delegates the actual update to:

```bash
omarchy plugin update local.omarchy-citizen --yes
```

This preserves Omarchy's own fast-forward, manifest validation and rollback behavior.

## Commands

```text
omarchy-citizen-backend status [--json]
omarchy-citizen-backend check [--json]
omarchy-citizen-backend update
omarchy-citizen-backend tick
omarchy-citizen-backend auto enable
omarchy-citizen-backend auto disable
omarchy-citizen-backend auto status
omarchy-citizen-backend install-service
omarchy-citizen-backend uninstall-service
omarchy-citizen-backend self-sync
omarchy-citizen-backend version
```

## Update trust model

Automatic updates can only be enabled when the plugin directory is a Git checkout.
When the user enables automatic updates, the current `origin` URL is stored as the
trusted remote. Future unattended updates are refused if:

- the Git remote changes,
- the checkout contains local changes,
- the branch has diverged,
- Omarchy validation fails,
- another update is already running.

## Scheduling

`auto enable` creates/enables a systemd **user** timer. It checks approximately every
six hours with a randomized delay. No root service and no localhost HTTP server are used.

## State

```text
~/.config/omarchy-citizen/updater.json
~/.local/state/omarchy-citizen/updater.log
~/.local/state/omarchy-citizen/update.lock
~/.local/lib/omarchy-citizen/omarchy-citizen-backend
```

The external binary location is intentional: the updater can keep running while the
plugin Git checkout itself is fast-forwarded. After a successful plugin update it copies
the newly bundled backend binary into the external location atomically.

## Building

```bash
cd backend
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -trimpath -ldflags='-s -w' \
  -o bin/omarchy-citizen-backend \
  ./cmd/omarchy-citizen-backend
```

The repository should commit the resulting binary so users do not need a Go toolchain.
