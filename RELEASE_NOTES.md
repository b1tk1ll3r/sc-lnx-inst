# Citizen Launcher 0.9.0 — distro-neutral preview

This release turns Omarchy Citizen into an optional integration instead of the product core.

## New

- standalone distro-neutral `citizen-launcher` Go binary
- embedded polished local GUI (`citizen-launcher gui`)
- Debian/Ubuntu `.deb` packaging
- generic Linux amd64 tarball
- Fedora RPM spec template
- distro detection via `/etc/os-release`
- package-manager detection for apt/dnf/pacman/zypper/apk diagnostics
- XDG-native config/data/state/cache paths with legacy Omarchy-Citizen migration
- generic systemd user maintenance timer, with launch-time maintenance fallback on non-systemd desktops
- support bundles generated entirely in Go
- Omarchy moved to `integrations/omarchy/`
- system tools preferred; LUG AppImage runtime is only a lazy portability fallback when cabextract/curl/unzip are unavailable

## Validation performed

- `go test ./...`
- `go vet ./...`
- static Linux amd64 Go build
- shell syntax verification
- embedded GUI HTTP/API smoke test
- generic user installer smoke test in an isolated HOME
- Omarchy updater integration test
- Debian package build and metadata inspection

## Still needs real-hardware testing

- clean Debian/Ubuntu desktop → RSI login/install → Star Citizen launch
- Fedora/openSUSE hardware testing
- AMD/NVIDIA/Intel Vulkan permutations
- production standalone self-update release channel once an official repository/release URL is chosen
