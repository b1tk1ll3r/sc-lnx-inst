package main

import (
	"archive/tar"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"citizen-launcher/backend/internal/signing"
)

func TestSignedChecksumsAreAuthoritative(t *testing.T) {
	seed, pub, err := signing.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	deb := strings.Repeat("a", 64)
	sums := []byte(deb + "  citizen-launcher_1.2.3_amd64.deb\n")
	sig, err := signing.Sign(seed, sums)
	if err != nil {
		t.Fatal(err)
	}
	newRel := func() githubRelease {
		return githubRelease{TagName: "v1.2.3", Assets: []githubAsset{
			{Name: "citizen-launcher_1.2.3_amd64.deb"},
			{Name: "unsigned-extra.tar.gz", Digest: "sha256:" + strings.Repeat("b", 64)},
		}}
	}
	rel := newRel()
	if err := applyVerifiedChecksums(&rel, sums, []byte(sig), []string{pub}); err != nil {
		t.Fatal(err)
	}
	if rel.Assets[0].Digest != "sha256:"+deb {
		t.Fatalf("signed digest not applied: %q", rel.Assets[0].Digest)
	}
	if rel.Assets[1].Digest != "" {
		t.Fatalf("asset outside the signed manifest kept a digest: %q", rel.Assets[1].Digest)
	}

	rel = newRel()
	tampered := []byte(strings.Repeat("c", 64) + "  citizen-launcher_1.2.3_amd64.deb\n")
	if err := applyVerifiedChecksums(&rel, tampered, []byte(sig), []string{pub}); err == nil {
		t.Fatal("tampered manifest accepted")
	}

	_, otherPub, _ := signing.GenerateKey()
	rel = newRel()
	if err := applyVerifiedChecksums(&rel, sums, []byte(sig), []string{otherPub}); err == nil {
		t.Fatal("signature by untrusted key accepted")
	}

	rel = newRel()
	rel.Assets[0].Digest = "sha256:" + strings.Repeat("d", 64)
	if err := applyVerifiedChecksums(&rel, sums, []byte(sig), []string{pub}); err == nil {
		t.Fatal("API digest contradicting the signed manifest accepted")
	}
}

func TestUnsignedReleaseRejected(t *testing.T) {
	rel := githubRelease{TagName: "v1.2.3", Assets: []githubAsset{{Name: "SHA256SUMS.txt", BrowserDownloadURL: "https://example.invalid/x"}}}
	if err := applySignedChecksums(&rel); err == nil || !strings.Contains(err.Error(), "not signed") {
		t.Fatalf("unsigned release not rejected: %v", err)
	}
}

func TestRequireHTTPS(t *testing.T) {
	for _, good := range []string{
		"https://github.com/owner/repo/releases/download/v1.2.3/x.deb",
		"https://install.robertsspaceindustries.com/rel/2/latest.yml",
	} {
		if err := requireHTTPS(good); err != nil {
			t.Fatalf("https URL rejected: %q: %v", good, err)
		}
	}
	for _, bad := range []string{
		"http://github.com/x",
		"ftp://example.com/x",
		"file:///etc/passwd",
		"https://user:pw@example.com/x",
		"//example.com/x",
		"",
	} {
		if err := requireHTTPS(bad); err == nil {
			t.Fatalf("insecure URL accepted: %q", bad)
		}
	}
}

func TestDownloadRefusesPlainHTTP(t *testing.T) {
	err := download("http://127.0.0.1:1/never", filepath.Join(t.TempDir(), "x"))
	if !errors.Is(err, errInsecureURL) {
		t.Fatalf("plain HTTP download not refused: %v", err)
	}
}

func TestVerifyReleaseDigestFailsClosed(t *testing.T) {
	p := filepath.Join(t.TempDir(), "asset")
	if err := os.WriteFile(p, []byte("abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, digest := range []string{"", "md5:900150983cd24fb0d6963f7d28e17f72", "sha256:", "sha256:abc"} {
		if err := verifyReleaseDigest(p, digest); err == nil {
			t.Fatalf("digest %q treated as verified", digest)
		}
	}
	if err := verifyReleaseDigest(p, "sha256:ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"); err != nil {
		t.Fatalf("correct digest rejected: %v", err)
	}
}

func TestVerifySHA512RequiresChecksum(t *testing.T) {
	p := filepath.Join(t.TempDir(), "setup.exe")
	if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifySHA512Base64(p, "  "); err == nil {
		t.Fatal("missing SHA-512 accepted")
	}
}

func TestExtractTarRejectsSymlinkChainEscape(t *testing.T) {
	// Each link passes a purely textual check, but together they resolve to the
	// parent of the extraction root.
	archive := makeTarGz(t, []tarTestEntry{
		{name: "sub/x", typ: tar.TypeSymlink, link: ".."},
		{name: "y", typ: tar.TypeSymlink, link: "sub/x/.."},
	})
	if err := extractTarArchiveSafe(archive, t.TempDir()); err == nil {
		t.Fatal("symlink chain escaping the extraction root was accepted")
	}
}

func TestStableVersionPattern(t *testing.T) {
	for _, good := range []string{"1.1.3", "10.0.12"} {
		if !stableVersionPattern.MatchString(good) {
			t.Fatalf("stable version rejected: %q", good)
		}
	}
	for _, bad := range []string{"", "1.1", "1.1.3a", "1.2.0-rc1", "1.1.3/../../x", "v1.1.3"} {
		if stableVersionPattern.MatchString(bad) {
			t.Fatalf("unstable/unsafe version accepted: %q", bad)
		}
	}
}

func TestReleaseAssetRejectsPathNames(t *testing.T) {
	a := &App{}
	rel := githubRelease{TagName: "v1.2.3", Assets: []githubAsset{
		{Name: "../citizen-launcher_1.2.3_amd64.deb"},
		{Name: "citizen-launcher_1.2.3_amd64.deb"},
	}}
	got, err := a.releaseAssetForMode(rel, "1.2.3", "deb")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "citizen-launcher_1.2.3_amd64.deb" {
		t.Fatalf("unexpected asset selected: %q", got.Name)
	}
}

func TestSupportSanitizerJSONKeys(t *testing.T) {
	in := []byte(`{"access_token":"tok123secret","nickname":"PilotHandle","session_id":"abcdef123"}`)
	out := string(sanitizeSupport(in, "/home/tester"))
	for _, secret := range []string{"tok123secret", "PilotHandle", "abcdef123"} {
		if strings.Contains(out, secret) {
			t.Fatalf("sanitizer leaked %q in %q", secret, out)
		}
	}
}
