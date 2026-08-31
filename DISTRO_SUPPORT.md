# Distribution support

Citizen Launcher deliberately avoids distro-specific Wine packages so the same tested gaming stack can run across distributions.

| Distribution family | Primary install | Background maintenance | Status |
|---|---|---|---|
| Debian 13 / Ubuntu / Mint | `.deb` | systemd user stack timer + system package self-update timer | primary |
| Fedora | generic binary / `install.sh` (RPM spec included) | systemd user timer | supported core |
| Arch Linux / Omarchy | `install.sh` | systemd user timer | supported core; optional Omarchy adapter |
| openSUSE | generic binary / `install.sh` | systemd user timer | supported core |
| other amd64 desktop Linux | generic binary | systemd timer when available | best effort |

Runtime requirements are an x86-64 CPU with AVX, a real Vulkan-capable GPU/driver, a Linux-native executable filesystem for the prefix, sufficient RAM+swap and storage, and basic desktop utilities. Citizen Launcher checks these before mutating the game stack.

The launcher detects common package managers for diagnostics but does not silently run full distribution upgrades or replace GPU/kernel packages.
