# Citizen Launcher 1.0.1

Reliability hotfix based on the first real-machine 1.0.0 acceptance bundle.

## Fixed

- Vulkan hardware detection now evaluates individual `vulkaninfo --summary` GPU blocks instead of treating the presence of any software ICD as a software-only system.
- A real AMD/NVIDIA/Intel Vulkan GPU is accepted even when Mesa also exposes `llvmpipe`/lavapipe/softpipe.
- Discrete GPUs are preferred for display/status, then integrated GPUs, then non-software virtual GPUs.
- QEMU/bochs remains blocked when it is the only usable graphics path, but no longer blocks a machine that also exposes a real working Vulkan GPU.
- Vulkan API compatibility is evaluated on the selected real GPU rather than the first arbitrary device in the summary.
- PowerShell compatibility self-test no longer requires stdout to be relayed through Wine. PowerShell Core is tested directly and the RSI-compatible wrapper is tested by its propagated exit status.
- Game/support status now discovers Prefix, DXVK, PowerShell, RSI Launcher and game files before applying an overall hardware/preflight block. A blocked machine therefore no longer reports existing components as falsely `missing`.

## Regression coverage

- AMD RADV + llvmpipe must select AMD and report Vulkan ready.
- llvmpipe-only must remain rejected.
- a successful PowerShell wrapper with empty stdout must pass.
- blocked overall health must still expose real installed component states.
- existing 1.0 release-gate coverage remains active: race detector, stack/GUI locks, desktop-entry repair, safe archive extraction, self-update validation, package validation and integration tests.

## Upgrade

Debian/Ubuntu/Mint:

```bash
sudo apt install ./citizen-launcher_1.0.1_amd64.deb
```

Existing configuration, Wine prefix, RSI Launcher and Star Citizen data are preserved.
