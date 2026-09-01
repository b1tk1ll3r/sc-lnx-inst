# Distribution support

Citizen Launcher keeps Wine, DXVK and RSI in the user account and deliberately avoids distro Wine packages. Native packages are thin integration layers around the same static Go core.

## Supported families

| Family | Examples | Preferred package | Automatic launcher update | Support level |
|---|---|---|---|---|
| Debian/Ubuntu | Debian 13+, Ubuntu 24.04/26.04, Mint, Pop!_OS, Zorin, TUXEDO OS | `.deb` | APT via verified system timer | primary |
| Fedora/RHEL | Fedora 43+, Nobara, Rocky/Alma/RHEL derivatives | `.rpm` | verified RPM via system timer | primary core |
| Arch | Arch, Manjaro, EndeavourOS, CachyOS, Garuda, Omarchy | `.pkg.tar.zst` | pacman via verified system timer | primary core |
| openSUSE | Tumbleweed, Slowroll; Leap where current dependencies are available | `.rpm` | verified RPM via system timer | primary core |
| Immutable desktops | Fedora Silverblue/Kinoite/Bazzite, SteamOS, openSUSE Aeon/MicroOS | `install.sh` user mode | verified user binary update | supported without mutating base image |
| Other glibc x86-64 desktop Linux | Gentoo, Void/glibc, custom distributions | `install.sh` / generic tarball | verified user binary update | best effort |

The game still requires an x86-64 CPU with AVX, a real Vulkan-capable GPU, sufficient RAM+swap/storage, a Linux-native executable filesystem and a Wine runner compatible with the host glibc/CPU. Citizen Launcher validates those conditions before changing the game stack.

## Package-manager mapping

Runtime detection uses `/etc/os-release` first and then the native package manager:

- Debian family → `apt`
- Fedora family → `dnf5` / `dnf`
- Arch family → `pacman`
- SUSE family → `zypper`
- Fedora Atomic/OSTree → `rpm-ostree` (base-image updates remain external)
- openSUSE transactional variants → `transactional-update` (base-image updates remain external)

Foreign package tools in `$PATH` do not override the detected distro family.

## One-command project installer

From an extracted project/release bundle:

```bash
./INSTALLIEREN.sh
```

It prefers the native package when it is present in `dist/`. On immutable systems, or when a native package is not available, it intentionally falls back to the user installation instead of modifying the base OS outside its package manager.

## Native package build inputs

- Debian: `packaging/build-deb.sh`
- Fedora/openSUSE RPM: `packaging/build-rpm.sh`
- Arch/pacman: `packaging/build-arch.sh`
- Generic Linux: `packaging/build-tarball.sh`
- Available formats on the current build host: `packaging/build-all.sh`

Gitea Actions builds RPM and Arch packages inside native Fedora and Arch container jobs on CI/release runs.

## Immutable distributions

Citizen Launcher never runs `rpm -U`/`pacman -U` against an immutable base image. When an installed system package is detected on an immutable host, the launcher reports that the host image/package layer owns that update. A `~/.local` user installation remains fully self-updateable from the verified generic tarball.

## Not a base-system updater

Citizen Launcher does not silently upgrade kernels, Mesa/NVIDIA drivers, firmware or the whole Linux distribution. Those remain owned by APT/DNF/pacman/zypper/rpm-ostree/transactional-update. This avoids partial upgrades and keeps rollback/recovery in the distribution's control.
