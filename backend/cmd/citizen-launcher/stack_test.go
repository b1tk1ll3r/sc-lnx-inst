package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseRSILatestElectronBuilder(t *testing.T) {
	y := `version: 2.15.0
files:
  - url: RSI Launcher-Setup-2.15.0.exe
    sha512: nested-sha
    size: 123
path: RSI Launcher-Setup-2.15.0.exe
sha512: top-sha
releaseDate: '2026-07-27T18:00:00.000Z'
`
	file, sha, err := parseRSILatestYML([]byte(y))
	if err != nil {
		t.Fatal(err)
	}
	if file != "RSI Launcher-Setup-2.15.0.exe" || sha != "top-sha" {
		t.Fatalf("unexpected: file=%q sha=%q", file, sha)
	}
}

func TestParseRSILatestNestedOnly(t *testing.T) {
	y := `version: 2.15.0
files:
  - url: "RSI Launcher-Setup-2.15.0.exe"
    sha512: "nested-sha"
`
	file, sha, err := parseRSILatestYML([]byte(y))
	if err != nil {
		t.Fatal(err)
	}
	if file != "RSI Launcher-Setup-2.15.0.exe" || sha != "nested-sha" {
		t.Fatalf("unexpected: file=%q sha=%q", file, sha)
	}
}

func TestParseRSILatestVersionFallback(t *testing.T) {
	file, _, err := parseRSILatestYML([]byte("version: 2.15.0\n"))
	if err != nil {
		t.Fatal(err)
	}
	if file != "RSI Launcher-Setup-2.15.0.exe" {
		t.Fatalf("unexpected file %q", file)
	}
}

func TestBasePrefixDoesNotUseFragilePowerShellMSI(t *testing.T) {
	for _, v := range basePrefixWinetricksVerbs {
		if strings.EqualFold(v, "powershell") {
			t.Fatalf("PowerShell must be managed by the verified portable compatibility layer, not Winetricks/MSI")
		}
	}
}

func TestWineRegistryKeyUsesSingleSeparators(t *testing.T) {
	if strings.Contains(wineFileAssociationsKey, `\\`) {
		t.Fatalf("registry key contains doubled separators: %q", wineFileAssociationsKey)
	}
	if wineFileAssociationsKey != `HKEY_CURRENT_USER\Software\Wine\FileOpenAssociations` {
		t.Fatalf("unexpected registry key: %q", wineFileAssociationsKey)
	}
}

func TestPowerShellStateRequiresFilesAndMarker(t *testing.T) {
	a := testApp(t)
	prefix := filepath.Join(a.home, "Games", "star-citizen")
	core, profile, wrapper64, wrapper32 := a.powerShellPaths(prefix)
	for _, p := range []string{core, profile, wrapper64, wrapper32} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, make([]byte, 2048), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if got := a.powerShellState(prefix); got != "repair" {
		t.Fatalf("without verification marker got %q", got)
	}
	if err := a.writePowerShellMarker(prefix, powerShellCoreVersion+"+wrapper-"+powerShellWrapperVersion); err != nil {
		t.Fatal(err)
	}
	if got := a.powerShellState(prefix); got != "ready" {
		t.Fatalf("managed compatibility layer got %q", got)
	}
	if err := os.Remove(wrapper64); err != nil {
		t.Fatal(err)
	}
	if got := a.powerShellState(prefix); got != "repair" {
		t.Fatalf("missing wrapper got %q", got)
	}
}

func TestPowerShellPinnedArtifactsUseSHA256(t *testing.T) {
	for name, sha := range map[string]string{
		"PowerShell Core":    powerShellCoreSHA256,
		"PowerShell wrapper": powerShellWrapperSHA256,
	} {
		if len(sha) != 64 {
			t.Fatalf("%s SHA-256 length=%d", name, len(sha))
		}
		for _, c := range sha {
			if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
				t.Fatalf("%s contains non-hex SHA-256 %q", name, sha)
			}
		}
	}
	if !strings.Contains(powerShellCoreURL, powerShellCoreVersion) || !strings.Contains(powerShellWrapperURL, powerShellWrapperVersion) {
		t.Fatal("pinned PowerShell URLs do not match pinned versions")
	}
	if powerShellCoreSHA256 != "cd62ad6d8174cc6fb85b335a0058444bc934fe27c39fa97fe342134286d28af9" {
		t.Fatal("PowerShell Core checksum drifted from upstream v7.4.19 release")
	}
	if powerShellWrapperSHA256 != "08f866265e0395f4bc5ddb18f3dff345771d039a62911e506c62084fd2533ec3" {
		t.Fatal("PowerShell wrapper checksum drifted from upstream v3.0.5 release")
	}
}

func TestExtractZipSafeRejectsTraversalAndSymlink(t *testing.T) {
	makeZip := func(name string, mode os.FileMode) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "test.zip")
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		zw := zip.NewWriter(f)
		h := &zip.FileHeader{Name: name, Method: zip.Store}
		h.SetMode(mode)
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte("payload"))
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		return path
	}
	if err := extractZipSafe(makeZip("../escape", 0o644), t.TempDir()); err == nil {
		t.Fatal("ZIP traversal was accepted")
	}
	if err := extractZipSafe(makeZip("link", os.ModeSymlink|0o777), t.TempDir()); err == nil {
		t.Fatal("ZIP symlink was accepted")
	}
}

func TestFormatCommandFailureHandlesMissingExpectedOutput(t *testing.T) {
	if got := formatCommandFailure(nil, nil); got == "" {
		t.Fatal("nil error and empty output should still yield a diagnostic")
	}
	if got := formatCommandFailure(nil, []byte("unexpected output")); got != "unexpected output" {
		t.Fatalf("unexpected diagnostic %q", got)
	}
}

func TestRequiredReleaseDigestFailsClosed(t *testing.T) {
	p := filepath.Join(t.TempDir(), "asset")
	if err := os.WriteFile(p, []byte("abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := requireReleaseDigest(p, ""); err == nil {
		t.Fatal("missing digest was accepted")
	}
	if err := requireReleaseDigest(p, "md5:deadbeef"); err == nil {
		t.Fatal("non-SHA256 digest was accepted")
	}
	if err := requireReleaseDigest(p, "sha256:ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"); err != nil {
		t.Fatalf("known SHA-256 rejected: %v", err)
	}
}

func TestVulkanHardwareSelectionIgnoresLLVMPipeWhenRealGPUExists(t *testing.T) {
	summary := `Devices:
========
GPU0:
    apiVersion         = 1.4.305
    deviceType         = PHYSICAL_DEVICE_TYPE_INTEGRATED_GPU
    deviceName         = AMD Radeon Graphics (RADV PHOENIX2)
GPU1:
    apiVersion         = 1.4.305
    deviceType         = PHYSICAL_DEVICE_TYPE_CPU
    deviceName         = llvmpipe (LLVM 19.1.7, 256 bits)
`
	dev, ok := selectVulkanHardwareDevice(summary)
	if !ok {
		t.Fatal("real Vulkan GPU was rejected because a software ICD was also present")
	}
	if dev.Name != "AMD Radeon Graphics (RADV PHOENIX2)" {
		t.Fatalf("selected GPU=%q", dev.Name)
	}
	if dev.APIMajor != 1 || dev.APIMinor != 4 {
		t.Fatalf("selected Vulkan version=%d.%d", dev.APIMajor, dev.APIMinor)
	}
}

func TestVulkanHardwareSelectionRejectsSoftwareOnly(t *testing.T) {
	summary := `Devices:
========
GPU0:
    apiVersion         = 1.4.305
    deviceType         = PHYSICAL_DEVICE_TYPE_CPU
    deviceName         = llvmpipe (LLVM 19.1.7, 256 bits)
`
	if dev, ok := selectVulkanHardwareDevice(summary); ok {
		t.Fatalf("software-only Vulkan device accepted: %#v", dev)
	}
}

func TestPowerShellProbeAcceptsSuccessfulWrapperWithoutStdout(t *testing.T) {
	a := testApp(t)
	prefix := filepath.Join(a.home, "Games", "star-citizen")
	gc := GameConfig{Prefix: prefix}
	core, _, wrapper64, _ := a.powerShellPaths(prefix)
	for _, p := range []string{core, wrapper64} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, make([]byte, 2048), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runner := filepath.Join(a.vendorDir, "wine", "fixture")
	bin := filepath.Join(runner, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	// The fixture intentionally emits no stdout: this reproduces the real Wine
	// behavior from the 1.0.0 support bundle.
	for _, name := range []string{"wine", "wineserver"} {
		p := filepath.Join(bin, name)
		if err := os.WriteFile(p, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(a.vendorDir, "wine"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(runner, filepath.Join(a.vendorDir, "wine", "current")); err != nil {
		t.Fatal(err)
	}
	if err := a.probePowerShell(gc, os.Environ()); err != nil {
		t.Fatalf("successful wrapper without stdout was rejected: %v", err)
	}
}

func TestGameStatusStillReportsInstalledComponentsWhenHealthIsBlocked(t *testing.T) {
	a := testApp(t)
	prefix := filepath.Join(a.home, "Games", "star citizen") // intentionally invalid path: forces blocked health
	gc := GameConfig{
		Prefix:      prefix,
		LauncherEXE: filepath.Join(prefix, "drive_c", "Program Files", "Roberts Space Industries", "RSI Launcher", "RSI Launcher.exe"),
		GameDir:     filepath.Join(prefix, "drive_c", "Program Files", "Roberts Space Industries", "StarCitizen"),
	}
	for _, p := range []string{
		filepath.Join(prefix, "drive_c"),
		filepath.Dir(gc.LauncherEXE),
		filepath.Join(gc.GameDir, "LIVE", "Bin64"),
	} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []string{
		filepath.Join(prefix, "system.reg"),
		filepath.Join(prefix, "user.reg"),
		gc.LauncherEXE,
		filepath.Join(gc.GameDir, "LIVE", "Bin64", "StarCitizen.exe"),
	} {
		if err := os.WriteFile(p, []byte("fixture"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.saveGameConfig(gc); err != nil {
		t.Fatal(err)
	}
	st := a.gameStatus()
	if st.Health != "hardware-blocked" {
		t.Fatalf("health=%q want hardware-blocked", st.Health)
	}
	if st.PrefixState != "ready" || st.LauncherState != "ready" || st.GameState != "ready" {
		t.Fatalf("blocked health hid real component state: prefix=%s launcher=%s game=%s", st.PrefixState, st.LauncherState, st.GameState)
	}
}
