# Automatic Citizen Launcher updates

Native mutable packages install `citizen-launcher-self-update.timer`. It checks after boot and roughly every six hours.

The release source is read from `/etc/citizen-launcher/release-repo`. Supported formats are:

- `github:owner/repository` (legacy `owner/repository` is also accepted)
- `gitea:https://gitea.example/api/v1/repos/owner/repository`

For privileged system updates, Gitea sources intentionally require HTTPS and the root-owned config file must not be group/world writable. Gitea workflow builds inject their own `${{ gitea.api_url }}` and repository automatically.

Every privileged update follows the same fail-closed chain:

1. read and validate the trusted release source;
2. resolve the newest stable release;
3. select only the package format matching the installed package database;
4. require SHA-256 metadata (GitHub API digest or the Gitea release `SHA256SUMS.txt`);
5. download to `/var/cache/citizen-launcher`;
6. verify SHA-256;
7. inspect package name, upstream version and x86-64 architecture;
8. install through the native package database;
9. verify that the installed version actually advanced.

Native modes:

- Debian/Ubuntu/Mint: `.deb` → APT
- Fedora/openSUSE/RHEL family: `.rpm` → RPM database
- Arch family: `.pkg.tar.zst` → pacman
- generic `~/.local`: verified tarball → atomic user-binary replacement

Immutable OSTree/transactional/SteamOS-style base systems are never modified by the automatic system updater. Their host image owns `/usr`; user-mode Citizen Launcher remains self-updateable.

An update never forcibly kills the running GUI. The old process detects that the on-disk/package version is newer and offers a controlled restart.
