package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestSelectLUGAppImage(t *testing.T) {
	assets := []githubAsset{
		{Name: "lug-helper-v4.16.tar.gz"},
		{Name: "lug-helper-v4.16.appimage"},
		{Name: "lug-helper-v4.16.appimage.zsync"},
	}
	a, err := selectAsset(assets, func(n string) bool {
		n = lower(n)
		return suffix(n, ".appimage") && contains(n, "lug-helper")
	})
	if err != nil || a.Name != "lug-helper-v4.16.appimage" {
		t.Fatalf("unexpected selection: %#v %v", a, err)
	}
}

func TestSelectStableWine(t *testing.T) {
	assets := []githubAsset{
		{Name: "lug-wine-tkg-staging-git-11.16-1-x86_64.tar.xz"},
		{Name: "lug-wine-tkg-git-11.16-1-x86_64.tar.xz"},
		{Name: "SHA256SUMS"},
	}
	a, err := selectAsset(assets, func(n string) bool {
		n = lower(n)
		if contains(n, "staging") || contains(n, "checksum") || contains(n, "sha") {
			return false
		}
		return contains(n, "lug-wine-tkg-git") && suffix(n, ".tar.xz")
	})
	if err != nil || a.Name != "lug-wine-tkg-git-11.16-1-x86_64.tar.xz" {
		t.Fatalf("unexpected selection: %#v %v", a, err)
	}
}

// tiny wrappers keep tests readable without importing strings again
func lower(s string) string     { return strings.ToLower(s) }
func suffix(s, p string) bool   { return strings.HasSuffix(s, p) }
func contains(s, p string) bool { return strings.Contains(s, p) }

func TestFormatCommandFailureIncludesProcessError(t *testing.T) {
	err := fmt.Errorf("signal: illegal instruction")
	got := formatCommandFailure(err, nil)
	if !strings.Contains(got, "illegal instruction") {
		t.Fatalf("process error was lost: %q", got)
	}
}

func TestWineRejectionRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rejected.json")
	in := map[string]wineRejection{
		"11.16-1": {Version: "11.16-1", Reason: "signal: illegal instruction", Fingerprint: "cpu-a"},
	}
	if err := writeWineRejections(path, in); err != nil {
		t.Fatal(err)
	}
	out := readWineRejections(path)
	if out["11.16-1"].Reason != in["11.16-1"].Reason {
		t.Fatalf("round trip mismatch: %#v", out)
	}
}
