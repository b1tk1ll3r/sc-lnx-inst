package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"0.9.2", "0.9.1", 1},
		{"v1.0.0", "0.9.9", 1},
		{"0.9.2", "0.9.2", 0},
		{"0.9.1", "0.9.2", -1},
		{"0.9.2-1", "0.9.2", 0},
	}
	for _, tc := range cases {
		got := compareVersions(tc.a, tc.b)
		if got != tc.want {
			t.Fatalf("compareVersions(%q,%q)=%d want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestReleaseAssetForDeb(t *testing.T) {
	a, _ := newApp()
	r := githubRelease{TagName: "v0.9.2", Assets: []githubAsset{
		{Name: "citizen-launcher-0.9.2-linux-amd64.tar.gz"},
		{Name: "citizen-launcher_0.9.2_amd64.deb", Digest: "sha256:abc"},
	}}
	asset, err := a.releaseAssetForMode(r, "0.9.2", "deb")
	if err != nil {
		t.Fatal(err)
	}
	if asset.Name != "citizen-launcher_0.9.2_amd64.deb" {
		t.Fatalf("wrong asset: %s", asset.Name)
	}
}

func TestVerifyDebPackageMetadata(t *testing.T) {
	if _, err := exec.LookPath("dpkg-deb"); err != nil {
		t.Skip("dpkg-deb unavailable")
	}
	root := t.TempDir()
	pkg := filepath.Join(root, "pkg")
	if err := os.MkdirAll(filepath.Join(pkg, "DEBIAN"), 0o755); err != nil {
		t.Fatal(err)
	}
	control := "Package: citizen-launcher\nVersion: 0.9.2\nArchitecture: amd64\nMaintainer: test\nDescription: test\n"
	if err := os.WriteFile(filepath.Join(pkg, "DEBIAN", "control"), []byte(control), 0o644); err != nil {
		t.Fatal(err)
	}
	deb := filepath.Join(root, "citizen-launcher_0.9.2_amd64.deb")
	cmd := exec.Command("dpkg-deb", "--build", "--root-owner-group", pkg, deb)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("dpkg-deb: %v: %s", err, out)
	}
	if err := verifyDebPackage(deb, "0.9.2"); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyDebPackageRejectsMismatchedNewerVersion(t *testing.T) {
	if _, err := exec.LookPath("dpkg-deb"); err != nil {
		t.Skip("dpkg-deb unavailable")
	}
	root := t.TempDir()
	pkg := filepath.Join(root, "pkg")
	if err := os.MkdirAll(filepath.Join(pkg, "DEBIAN"), 0o755); err != nil {
		t.Fatal(err)
	}
	control := "Package: citizen-launcher\nVersion: 9.9.9\nArchitecture: amd64\nMaintainer: test\nDescription: test\n"
	if err := os.WriteFile(filepath.Join(pkg, "DEBIAN", "control"), []byte(control), 0o644); err != nil {
		t.Fatal(err)
	}
	deb := filepath.Join(root, "citizen-launcher_1.0.0_amd64.deb")
	if out, err := exec.Command("dpkg-deb", "--build", "--root-owner-group", pkg, deb).CombinedOutput(); err != nil {
		t.Fatalf("dpkg-deb: %v: %s", err, out)
	}
	if err := verifyDebPackage(deb, "1.0.0"); err == nil {
		t.Fatal("mismatched package version was accepted")
	}
}
