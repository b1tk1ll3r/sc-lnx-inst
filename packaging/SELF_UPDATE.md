# Automatic launcher updates

## Debian / Ubuntu / Mint

The `.deb` installs a root systemd timer:

- `citizen-launcher-self-update.timer`
- checks on boot and every six hours
- reads the latest GitHub Release from the configured repository
- accepts only a newer `citizen-launcher_<version>_amd64.deb`
- requires GitHub's `sha256:` release asset digest
- verifies the downloaded package name, version, and architecture with `dpkg-deb`
- installs via `apt-get` with the dpkg lock timeout enabled

The release repository is stored in:

```text
/etc/citizen-launcher/release-repo
```

Default for this project:

```text
sendnwv/omarchy-sc
```

A package update does not forcibly terminate a running GUI. The existing GUI detects
that `/usr/bin/citizen-launcher` is newer and offers **Launcher neu starten**.

## Generic user install

The Autopilot user timer checks GitHub releases. For a `~/.local` installation it
can replace the launcher binary atomically from the verified Linux tarball without
root privileges.

## Release publishing

`.github/workflows/release.yml` builds and publishes the `.deb`, generic tarball and
SHA256SUMS whenever a `v<VERSION>` tag is pushed. GitHub computes an immutable asset
digest that the Debian self-updater verifies before installation.
