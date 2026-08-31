# Development status — 0.8.1

This archive is a **development preview**, not a claim that Star Citizen has
been end-to-end validated on real gaming hardware yet.

## Verified in this build

- Shell scripts pass `bash -n`.
- `citizenctl` has a fake-backend regression test covering status, full setup, launch, repair and updater routing.
- GitHub Actions CI runs shell checks, Go tests/vet/build and the adapter smoke test.
- Go unit tests pass.
- Go backend builds as version `0.8.1`.
- Git-plugin updater integration test passes.
- Backend contains the owned `game-status`, `game-install`, `game-launch`, and
  `game-repair` paths.
- Hardware gate detects the QEMU/Bochs test environment that was found during
  development instead of looping through Wine repairs.
- Wine compatibility selection can reject newer runners and retain a compatible
  fallback.
- LUG is optional and cannot block the owned install path.

## Still needs real-hardware validation

The following must be tested on an Omarchy machine with a real Vulkan-capable
AMD/NVIDIA/Intel GPU before calling 0.8.1 a production release:

- fresh Wine prefix creation with the selected runner
- current Winetricks verbs (`arial`, `tahoma`, `powershell`, `win11`) plus separately managed DXVK
- current RSI Launcher unattended installation
- first RSI login and game download
- actual Star Citizen launch and EAC behavior
- DXVK update/rollback on a populated prefix
- NGL installation on the owned prefix
- upgrade from an existing LUG-managed installation containing real game data

## Maintenance contract

To keep future maintenance manageable, the UI must not depend on Wine/DXVK/RSI
version-specific paths. Upstream details belong in the Go backend. The QML panel
should consume only backend state/actions.

LUG documentation may be consulted for compatibility guidance, but its installer
is not part of the critical runtime contract.
