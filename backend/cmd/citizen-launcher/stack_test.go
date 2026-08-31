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
