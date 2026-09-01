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

func TestReleaseAssetsForAllNativeModes(t *testing.T) {
	a, _ := newApp()
	r := githubRelease{TagName: "v1.1.1", Assets: []githubAsset{
		{Name: "citizen-launcher_1.1.1_amd64.deb"},
		{Name: "citizen-launcher-1.1.1-1.linux.x86_64.rpm"},
		{Name: "citizen-launcher-1.1.1-1-x86_64.pkg.tar.zst"},
		{Name: "citizen-launcher-1.1.1-linux-amd64.tar.gz"},
	}}
	want := map[string]string{
		"deb":  "citizen-launcher_1.1.1_amd64.deb",
		"rpm":  "citizen-launcher-1.1.1-1.linux.x86_64.rpm",
		"arch": "citizen-launcher-1.1.1-1-x86_64.pkg.tar.zst",
		"user": "citizen-launcher-1.1.1-linux-amd64.tar.gz",
	}
	for mode, name := range want {
		asset, err := a.releaseAssetForMode(r, "1.1.1", mode)
		if err != nil {
			t.Fatalf("%s: %v", mode, err)
		}
		if asset.Name != name {
			t.Fatalf("%s: got %q want %q", mode, asset.Name, name)
		}
	}
}

func TestVerifyPackageIdentityAcceptsNativeReleaseSuffixes(t *testing.T) {
	cases := []struct{ version, arch, wantArch string }{
		{"1.1.0", "amd64", "amd64"},
		{"1.1.0-1", "x86_64", "x86_64"},
		{"1.1.0", "x86_64", "x86_64"},
	}
	for _, tc := range cases {
		if err := verifyPackageIdentity("citizen-launcher", tc.version, tc.arch, "1.1.0", tc.wantArch); err != nil {
			t.Fatal(err)
		}
	}
	if err := verifyPackageIdentity("evil", "1.1.0", "x86_64", "1.1.0", "x86_64"); err == nil {
		t.Fatal("wrong package name accepted")
	}
}

func TestNativePackageDatabaseParsers(t *testing.T) {
	root := t.TempDir()
	write := func(name, body string) {
		p := filepath.Join(root, name)
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write("dpkg-query", "printf 'install ok installed\\n1.1.0\\n'")
	write("rpm", "printf '1.1.0\\n'")
	write("pacman", "printf 'citizen-launcher 1.1.0-1\\n'")
	t.Setenv("PATH", root)

	for mode, want := range map[string]string{"deb": "1.1.0", "rpm": "1.1.0", "arch": "1.1.0-1"} {
		pkg := queryInstalledPackage(mode)
		if pkg.Mode != mode || pkg.Version != want {
			t.Fatalf("%s parser: %#v want version %s", mode, pkg, want)
		}
	}
}

func TestParseLauncherReleaseSource(t *testing.T) {
	cases := []struct {
		in, kind string
		ok       bool
	}{
		{"owner/repo", "github", true},
		{"github:owner/repo", "github", true},
		{"gitea:https://git.example.test/api/v1/repos/owner/repo", "gitea", true},
		{"gitea:https://git.example.test/sub/path/api/v1/repos/owner/repo", "gitea", true},
		{"gitea:https://git.example.test/api/v1/repos/owner/repo/", "gitea", true},
		{"gitea:http://git.example.test/api/v1/repos/owner/repo", "", false},
		{"gitea:https://user:pass@git.example.test/api/v1/repos/owner/repo", "", false},
		{"gitea:https://git.example.test/owner/repo", "", false},
		{"../evil", "", false},
	}
	for _, tc := range cases {
		src, ok := parseLauncherReleaseSource(tc.in)
		if ok != tc.ok {
			t.Fatalf("parseLauncherReleaseSource(%q) ok=%v want %v", tc.in, ok, tc.ok)
		}
		if ok && src.Kind != tc.kind {
			t.Fatalf("parseLauncherReleaseSource(%q) kind=%q want %q", tc.in, src.Kind, tc.kind)
		}
	}
}

func TestParseSHA256SUMS(t *testing.T) {
	body := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa  citizen-launcher_1.1.1_amd64.deb\n" +
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb *citizen-launcher-1.1.1-linux-amd64.tar.gz\n" +
		"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc  ../escape.deb\n" +
		"not-a-hash  ignored\n"
	got := parseSHA256SUMS(body)
	if got["citizen-launcher_1.1.1_amd64.deb"] != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatal("deb checksum missing")
	}
	if got["citizen-launcher-1.1.1-linux-amd64.tar.gz"] != "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" {
		t.Fatal("tarball checksum missing")
	}
	if _, ok := got["escape.deb"]; ok {
		t.Fatal("path-traversal checksum entry accepted")
	}
}
