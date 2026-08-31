# Distribution support

Citizen Launcher deliberately avoids distro-specific Wine packages.

| Distribution family | Install method | Autopilot | Notes |
|---|---|---|---|
| Debian / Ubuntu / Mint | `.deb` or `install.sh` | systemd user timer | GPU/Vulkan driver must be working |
| Fedora | generic tarball / `install.sh` | systemd user timer | RPM spec included |
| Arch / Omarchy | `install.sh` | systemd user timer | optional Omarchy bar integration |
| openSUSE | generic tarball / `install.sh` | systemd user timer | package-manager independent core |
| non-systemd desktop | generic install | maintenance on app use | timer is skipped |

The launcher detects `apt`, `dnf`, `pacman`, `zypper` and `apk` for diagnostics/UI, but does not silently alter the base OS.
