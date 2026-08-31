# Citizen Launcher 1.0.0

Citizen Launcher is a distro-neutral Star Citizen setup, launch, repair and maintenance application for Linux. Omarchy support is optional; the core is one static Go binary shared by Debian/Ubuntu, Fedora, Arch/Omarchy and other desktop distributions.

## Product goal

**Install Citizen Launcher → click setup → log into RSI → install/play Star Citizen.**

The launcher owns the fragile user-space gaming stack so users do not have to pick Wine builds, copy DXVK DLLs or rebuild prefixes by hand. Kernel, GPU driver and base-distribution updates remain owned by the Linux distribution.

## What 1.0 manages

- hardware/Vulkan/AVX/RAM/storage/filesystem preflight
- Linux `vm.max_map_count` and file-limit preparation
- newest **locally compatible** stable LUG Wine runner with rollback retention
- isolated Wine self-tests before runner activation
- deterministic, SHA-256-pinned Winetricks base setup
- verified portable PowerShell Core + RSI-compatible Wine wrapper (no fragile PowerShell MSI install)
- DXVK download, digest verification, installation and native DLL overrides
- RSI `latest.yml`, SHA-512 verified installer download and launcher repair
- Star Citizen desktop entry and stable launch path
- single-instance GUI, stack-operation locking and duplicate RSI/Star Citizen launch prevention
- automatic gaming-stack maintenance
- automatic verified Debian package self-updates
- privacy-conscious support bundle and rotating logs
- migration from older Omarchy Citizen / user-local installations

## Install on Debian / Ubuntu / Mint

Install the release `.deb`:

```bash
sudo apt install ./citizen-launcher_1.0.0_amd64.deb
```

The package adds the desktop application and enables the system package-update timer. Future Citizen Launcher `.deb` releases can be installed automatically after release-asset digest and package metadata verification.

## Generic desktop Linux install

```bash
./install.sh
```

or use the release tarball. Fedora/Arch/openSUSE users can use the generic build; an RPM spec is included for packaging work.

For Omarchy plus the optional bar widget:

```bash
./install-omarchy.sh
```

## GUI

```bash
citizen-launcher gui
```

The GUI is embedded in the binary and served only on `127.0.0.1` behind a random per-process route. Chromium-family browsers open it as an app window; otherwise the default browser is used. A file lock guarantees a single GUI backend. Opening Citizen Launcher again reuses the existing instance.

## Data locations

- `~/.config/citizen-launcher`
- `~/.local/share/citizen-launcher`
- `~/.local/state/citizen-launcher`
- `~/.cache/citizen-launcher`
- default Wine prefix: `~/Games/star-citizen`

The uninstaller intentionally preserves the game prefix, game files, configuration and support logs unless the user removes them separately.

## Build and verification

```bash
./tests/full-verify.sh
```

The full gate includes shell syntax, gofmt cleanliness, unit/regression tests, `go vet`, static amd64 build, Go race detector, Omarchy update integration test, Debian package build/metadata/payload verification and generic tarball verification.

## Security / reliability boundaries

- no passwordless sudo rules are created
- executable release assets fail closed when their expected digest is unavailable
- RSI installer is checked against its published SHA-512 metadata
- Winetricks and the portable RSI PowerShell compatibility assets are version-pinned and checksum-verified
- Wine/DXVK archives are extracted with path/symlink traversal checks
- privileged Debian self-update accepts only the configured release repository, expected package name/version/architecture and verified SHA-256 asset digest
- maintenance never rewrites the active Wine prefix while RSI Launcher/Star Citizen is using it
- incomplete prefixes are preserved instead of blindly deleted

See `ARCHITECTURE.md`, `DISTRO_SUPPORT.md` and `RELEASE_NOTES.md` for details.
