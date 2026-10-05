package signing

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSignVerifyRoundTrip(t *testing.T) {
	seed, pub, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	msg := []byte("abc  citizen-launcher_1.2.3_amd64.deb\n")
	sig, err := Sign(seed, msg)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyWith([]string{pub}, msg, []byte(sig+"\n")); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
	if err := VerifyWith([]string{pub}, append(msg, 'x'), []byte(sig)); err == nil {
		t.Fatal("tampered message accepted")
	}
	if err := VerifyWith([]string{pub}, msg, []byte("not-base64!")); err == nil {
		t.Fatal("malformed signature accepted")
	}
	if got, _ := PublicKeyFromSeed(seed); got != pub {
		t.Fatalf("public key derivation mismatch: %s != %s", got, pub)
	}
}

func TestEmbeddedKeysValid(t *testing.T) {
	if len(PublicKeys) == 0 {
		t.Fatal("no trusted release keys embedded")
	}
	for _, k := range PublicKeys {
		if _, err := PEM(k); err != nil {
			t.Fatal(err)
		}
	}
}

// install.sh verifies SHA256SUMS.txt with OpenSSL before any Go code runs;
// its embedded PEM must match the keys compiled into the launcher.
func TestInstallerEmbedsSameKeys(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	script := string(b)
	for _, k := range PublicKeys {
		p, _ := PEM(k)
		body := strings.TrimSpace(p)
		body = strings.TrimPrefix(body, "-----BEGIN PUBLIC KEY-----")
		body = strings.TrimSpace(strings.TrimSuffix(body, "-----END PUBLIC KEY-----"))
		if !strings.Contains(script, body) {
			t.Fatalf("install.sh does not embed release key %s", k)
		}
	}
}
