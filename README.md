# Citizen Launcher – Star Citizen on Linux

[![CI](https://github.com/b1tk1ll3r/sc-lnx-inst/actions/workflows/ci.yml/badge.svg)](https://github.com/b1tk1ll3r/sc-lnx-inst/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/b1tk1ll3r/sc-lnx-inst)](https://github.com/b1tk1ll3r/sc-lnx-inst/releases/latest)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

**Install it → click "Setup" → log in to RSI → play Star Citizen.**

Citizen Launcher installs, launches, repairs and maintains Star Citizen on desktop Linux. It manages the fragile parts of the gaming stack for you: the Wine runner, DXVK, the RSI Launcher and the Wine prefix. You never pick Wine builds, copy DLLs or rebuild prefixes by hand.

It is one static Go binary with an embedded GUI. Native packages are available for Debian/Ubuntu, Fedora/openSUSE and Arch, plus a portable build for everything else.

> Citizen Launcher is a community project. It is not affiliated with or endorsed by Cloud Imperium Games or Roberts Space Industries. Star Citizen® is a trademark of Cloud Imperium Rights LLC.

---

## Contents

- [Features](#features)
- [Requirements](#requirements)
- [Installation](#installation)
- [First start](#first-start)
- [Command line](#command-line)
- [Updates](#updates)
- [Security model](#security-model)
- [Files and directories](#files-and-directories)
- [Troubleshooting](#troubleshooting)
- [Uninstall](#uninstall)
- [Building from source](#building-from-source)
- [CI and releases](#ci-and-releases)
- [Project layout](#project-layout)
- [License](#license)

## Features

### Setup and launch

- Hardware preflight before anything changes: x86-64 with AVX, a real Vulkan GPU, RAM plus swap, free disk space, and a filesystem that allows executables.
- Sets `vm.max_map_count` and file-descriptor limits as Star Citizen expects them (asks once through Polkit).
- Picks the newest stable [LUG](https://github.com/starcitizen-lug) Wine runner that works on *your* system. Each candidate is tested in a throw-away prefix before it is activated. The previous working runner is kept for rollback.
- Winetricks base setup pinned to a checksum, DXVK with native DLL overrides, and a verified PowerShell compatibility layer.
- RSI Launcher installation from CIG's official `latest.yml`, verified with SHA-512.
- Desktop entry and a stable launch path for Star Citizen.

### Maintenance

- Autopilot keeps Wine and DXVK up to date. Updates are deferred while the game or RSI Launcher is running.
- One-click repair for a broken prefix, runner or launcher.
- A single GUI instance per user, operation locks, and protection against starting the game twice.
- A support bundle with logs, redacted for privacy.

### Distribution integration

- Native `.deb`, `.rpm` and `.pkg.tar.zst` packages, with a systemd timer that updates **only Citizen Launcher itself**, verified, through your package manager.
- Immutable systems (Silverblue/Kinoite/Bazzite, SteamOS, openSUSE Aeon/MicroOS) get a user-mode install in `~/.local`. The base image is never touched.
- An optional [Omarchy](https://omarchy.org) bar widget.

Kernel, GPU driver, Mesa and firmware updates stay with your distribution. Citizen Launcher never runs a full system upgrade.

## Requirements

| | Minimum |
|---|---|
| CPU | x86-64 with AVX |
| GPU | Vulkan-capable AMD, NVIDIA or Intel GPU with a current driver |
| Memory | 16 GiB RAM; RAM + swap/zram of at least 48 GiB recommended |
| Storage | ~150 GiB free on a Linux filesystem (ext4, btrfs, xfs) mounted with `exec`, SSD strongly recommended |
| OS | glibc-based 64-bit desktop Linux with systemd |

The launcher checks all of these itself and tells you what is missing.

## Installation

One command, for every supported distribution:

```bash
bash <(wget -qO- https://github.com/b1tk1ll3r/sc-lnx-inst/releases/latest/download/install.sh)
```

The installer:

1. detects your distribution family and whether the system is immutable;
2. downloads the latest release's `SHA256SUMS.txt` and checks its **Ed25519 signature** against the release key embedded in the installer (OpenSSL 3);
3. downloads the matching package and checks its SHA-256 against the signed list;
4. installs it with your package manager (`apt`, `dnf`, `zypper`, `pacman`). On immutable or unknown systems it installs to `~/.local` without root.

Nothing is installed if the signature or a checksum does not match. Without `wget`, use `bash <(curl -fsSL …)` with the same URL.

| Option | Effect |
|---|---|
| `--user` | user install in `~/.local` (no root), even on mutable systems |
| `--version X.Y.Z` | install a specific release instead of the latest |
| `--uninstall` | remove Citizen Launcher (game data stays) |
| `--local` | install `backend/bin/citizen-launcher` from a source checkout (after `./build.sh`) |

Options go after the command, e.g. `bash <(wget -qO- https://github.com/b1tk1ll3r/sc-lnx-inst/releases/latest/download/install.sh) --user`.

| Distribution family | Installed as |
|---|---|
| Debian 13+, Ubuntu 24.04+, Mint, Pop!_OS, Zorin, TUXEDO OS | `.deb` via APT |
| Fedora 43+, Nobara, RHEL-family | `.rpm` via DNF |
| openSUSE Tumbleweed / Slowroll | `.rpm` via zypper |
| Arch, Manjaro, EndeavourOS, CachyOS, Garuda, Omarchy | `.pkg.tar.zst` via pacman |
| Silverblue/Kinoite/Bazzite, SteamOS, Aeon/MicroOS, other glibc distributions | user install in `~/.local` |

### Manual download

All packages, `install.sh`, `SHA256SUMS.txt` and `SHA256SUMS.txt.sig` are attached to every [release](https://github.com/b1tk1ll3r/sc-lnx-inst/releases/latest). To verify by hand:

```bash
cat > release.pem <<'EOF'
-----BEGIN PUBLIC KEY-----
MCowBQYDK2VwAyEA2Ra7HkxMbqd4TlJFLosTXzzaC+sg0+CQgb/5VgQiG98=
-----END PUBLIC KEY-----
EOF
base64 -d SHA256SUMS.txt.sig > SHA256SUMS.txt.sig.bin
openssl pkeyutl -verify -pubin -inkey release.pem -rawin -in SHA256SUMS.txt -sigfile SHA256SUMS.txt.sig.bin
sha256sum -c --ignore-missing SHA256SUMS.txt
gh attestation verify <file> --repo b1tk1ll3r/sc-lnx-inst   # optional build provenance
```

### Optional: Omarchy bar widget

After installing Citizen Launcher, from a source checkout:

```bash
bash integrations/omarchy/install.sh
```

## First start

1. Open **Citizen Launcher** from your application menu, or run `citizen-launcher gui`.
2. Click **Setup**. The launcher checks your hardware, prepares the system limits (one Polkit prompt), installs Wine, DXVK and the RSI Launcher into `~/Games/star-citizen`, and tests everything.
3. Log in to the RSI Launcher and install Star Citizen as usual.
4. From then on, start the game from Citizen Launcher or the *Star Citizen* desktop entry.

The GUI is a local web app served only on `127.0.0.1`. It opens in a Chromium-based app window or your default browser through a single-use login link; after that, access requires a session cookie that never appears in a URL.

## Command line

```text
citizen-launcher gui                     open the GUI (reuses a running instance)
citizen-launcher game-status [--json]    show the state of prefix, runner, DXVK and RSI Launcher
citizen-launcher game-install            run the full setup
citizen-launcher game-launch             start the RSI Launcher / Star Citizen
citizen-launcher game-repair             repair runner, DXVK and launcher
citizen-launcher maintain                run one maintenance pass now
citizen-launcher doctor                  diagnostics
citizen-launcher platform [--json]       detected distribution, package mode, immutability
citizen-launcher prepare-system          apply vm.max_map_count and file limits (Polkit)
citizen-launcher autopilot enable|disable|status
citizen-launcher self-update check|status|apply [--system] [--quiet]
citizen-launcher support                 create a redacted support bundle in ~/Downloads
citizen-launcher --version
```

## Updates

| What | How |
|---|---|
| Wine runner, DXVK, RSI Launcher | Autopilot (systemd user timer). Every component is verified and tested before it is activated, and deferred while the game runs. |
| Citizen Launcher (native package) | `citizen-launcher-self-update.timer` checks about every 6 h and installs the new release through APT, DNF/RPM or pacman. |
| Citizen Launcher (user install) | Atomic replacement of `~/.local/bin/citizen-launcher` from the verified release tarball. |
| Kernel, GPU driver, Mesa, distro | Your distribution's tools. Citizen Launcher does not touch these. |

The update source is stored in `/etc/citizen-launcher/release-repo`, for example `github:b1tk1ll3r/sc-lnx-inst`. See [packaging/SELF_UPDATE.md](packaging/SELF_UPDATE.md).

## Security model

- **Signed releases.** `SHA256SUMS.txt` of every release is signed with the project's Ed25519 key. The self-updater and `install.sh` only install packages listed in a correctly signed manifest; a compromised GitHub account or CI token alone cannot ship an update.
- **Fail-closed verification.** Every downloaded executable must match a published checksum. Launcher packages use the signed `SHA256SUMS.txt`. Wine, DXVK and LUG use GitHub digests. Winetricks and PowerShell use checksums pinned in the source. The RSI installer uses the SHA-512 from `latest.yml`. If there is no checksum, the component is not installed.
- **HTTPS only.** Every download is HTTPS-only, and redirects to plain HTTP are refused. The RSI installer is accepted only from `install.robertsspaceindustries.com`. Downloads have size limits.
- **Package checks before root installs.** Before a privileged install, the package's name, version and architecture are checked against the release. Downgrades are refused, and only plain `X.Y.Z` release tags count as update targets.
- **Trusted update source.** As root, the updater reads the update source only from the root-owned `/etc/citizen-launcher/release-repo`. Environment overrides are ignored for root.
- **Sandboxed updater service.** The systemd self-update service runs with `ProtectHome`, `PrivateTmp`, `NoNewPrivileges` and kernel/cgroup protections.
- **Safe archive extraction.** Archives are unpacked in-process. Absolute paths, `..` and symlinks that escape the target, including chains of symlinks, are rejected. Setuid bits are stripped.
- **Local GUI protections.** The GUI listens only on 127.0.0.1. Browsers get in through a single-use login link (valid 2 minutes) that is exchanged for an `HttpOnly`, `SameSite=Strict` session cookie, so other local users cannot reuse a token from the process list. Host and Origin headers are checked against DNS rebinding and cross-site requests, and the CSP forbids inline scripts.
- **No passwordless sudo.** No sudo rules are created. Privileged steps go through Polkit, `pkexec`.

Found a vulnerability? Please open a [private security advisory](https://github.com/b1tk1ll3r/sc-lnx-inst/security/advisories/new) instead of a public issue.

## Files and directories

| Path | Content |
|---|---|
| `~/.config/citizen-launcher/` | configuration |
| `~/.local/share/citizen-launcher/` | managed Wine runners, DXVK, tools |
| `~/.local/state/citizen-launcher/` | logs |
| `~/.cache/citizen-launcher/` | download cache |
| `~/Games/star-citizen/` | default Wine prefix, including the game |
| `/etc/citizen-launcher/release-repo` | self-update source (native packages) |
| `/var/lib/citizen-launcher/`, `/var/cache/citizen-launcher/` | state and cache of the system updater |

Uninstalling or reinstalling the launcher never deletes the prefix or game data.

## Troubleshooting

- **Run `citizen-launcher doctor`** first. It reports hardware, driver, limit and prefix problems with concrete hints.
- **Repair** (GUI or `citizen-launcher game-repair`) re-validates the runner, DXVK and the RSI Launcher without deleting game data.
- **Support bundle**: `citizen-launcher support` writes `Citizen-Launcher-Support-*.tar.gz` (mode 0600) to your Downloads folder. Emails, IPs, MAC addresses, tokens, RSI handles and home paths are redacted. Please still check the bundle before sharing it.
- Updater logs for native installs: `journalctl -u citizen-launcher-self-update.service`.

## Uninstall

```bash
bash <(wget -qO- https://github.com/b1tk1ll3r/sc-lnx-inst/releases/latest/download/install.sh) --uninstall
```

This removes the native package or the user install. To remove the game as well, delete `~/Games/star-citizen` and the directories listed above.

## Building from source

Requirements: Go (current stable), bash. Optional: `dpkg-deb`, `rpmbuild`, `makepkg` for the native packages.

```bash
./build.sh                       # static linux/amd64 binary -> backend/bin/citizen-launcher
./packaging/build-all.sh         # every package format the host can build -> dist/
./tests/full-verify.sh           # full regression suite (Linux; needs python3-yaml, dpkg-dev, zstd, jq, openssl)
bash install.sh --local          # install your local build to ~/.local
```

Individual builders: `packaging/build-deb.sh`, `build-rpm.sh`, `build-arch.sh` (not as root) and `build-tarball.sh`. They package the existing `backend/bin/citizen-launcher`, so run `./build.sh` first. `CITIZEN_LAUNCHER_RELEASE_REPO=github:<owner>/<repo>` sets the self-update source for forks.

## CI and releases

GitHub Actions workflows live in [`.github/workflows/`](.github/workflows):

| Workflow | Trigger | What it does |
|---|---|---|
| `packages.yml` | reusable | Runs the regression suite (ShellCheck, gofmt, `go vet`, unit and race tests, integration test) and builds the static backend once. Packages it as `.deb` + tarball (Ubuntu), `.rpm` (Fedora 44 container) and `.pkg.tar.zst` (Arch container, unprivileged `makepkg`). Install-tests every package on Debian 13, Ubuntu 24.04, Fedora 44, openSUSE Tumbleweed and Arch. |
| `ci.yml` | push to `main`, pull requests | runs `packages.yml` |
| `release.yml` | tag `v*` | Checks that the tag matches `VERSION` and runs `packages.yml`. Then generates `SHA256SUMS.txt`, signs it (`SHA256SUMS.txt.sig`), creates build-provenance attestations, publishes the GitHub release with `install.sh`, and finally runs the documented one-liner against the published release on Debian, Ubuntu, Fedora, openSUSE and Arch. |

### Cutting a release

```bash
echo 1.2.0 > VERSION   # also update RELEASE_NOTES.md (used as the release body)
git commit -am "v1.2.0"
git tag v1.2.0
git push origin main v1.2.0
```

Tags with a suffix (e.g. `v1.2.0-rc1`) are published as pre-releases and are never offered to the automatic updater.

### Release signing key

Releases are signed with an Ed25519 key. The public key is compiled into the launcher (`backend/internal/signing/signing.go`) and embedded in `install.sh`; a test keeps both in sync. The private key is stored only as the GitHub Actions secret `RELEASE_SIGNING_KEY`. The publish job runs in the `release` environment, so you can also store it as an environment secret and add required reviewers.

```bash
cd backend
go run ./cmd/release-sign keygen ~/release-signing.key   # prints the new public key
```

To rotate, add the new public key to `PublicKeys` and to `RELEASE_PUBKEYS` in `install.sh` (keep the old one for a transition release), update the secret, and ship a release. Forks must use their own key.

## Project layout

```text
backend/cmd/citizen-launcher/   Go core: setup, launch, repair, maintenance, self-update, GUI (web/)
backend/cmd/release-sign/       release signing tool (keygen/sign/verify)
backend/internal/signing/       Ed25519 manifest signatures + trusted release keys
packaging/                      deb/rpm/arch/tarball builders, systemd units, desktop + AppStream files
integrations/omarchy/           optional Omarchy bar widget, panel and its install.sh
tests/                          shell regression and policy tests
.github/workflows/              CI and release pipelines
install.sh                      installer / uninstaller (also published with every release)
```

More background: [ARCHITECTURE.md](ARCHITECTURE.md), [DISTRO_SUPPORT.md](DISTRO_SUPPORT.md), [packaging/SELF_UPDATE.md](packaging/SELF_UPDATE.md) and [RELEASE_NOTES.md](RELEASE_NOTES.md).

## License

[MIT](LICENSE)
