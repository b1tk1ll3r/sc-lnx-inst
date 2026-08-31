package main

import (
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

func TestBasePrefixDoesNotRequirePowerShell(t *testing.T) {
	for _, v := range basePrefixWinetricksVerbs {
		if strings.EqualFold(v, "powershell") {
			t.Fatalf("PowerShell must remain optional")
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
