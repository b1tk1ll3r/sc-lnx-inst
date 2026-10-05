// Package signing signs and verifies release checksum manifests
// (SHA256SUMS.txt) with the project's Ed25519 release key.
//
// Signature format: base64 (standard encoding) of the raw 64-byte Ed25519
// signature over the exact manifest bytes, stored as SHA256SUMS.txt.sig. This
// is the same raw signature OpenSSL 3 produces/verifies with
// `openssl pkeyutl -rawin`, so install.sh can verify it without Go.
package signing

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
)

// PublicKeys are the trusted release keys (base64 of the raw 32-byte Ed25519
// public key). Keep the previous key here during a rotation so releases signed
// by either key verify. install.sh embeds the same key(s) as PEM; the
// signing tests keep both in sync.
var PublicKeys = []string{
	"2Ra7HkxMbqd4TlJFLosTXzzaC+sg0+CQgb/5VgQiG98=",
}

// ErrBadSignature is returned when no trusted key verifies the manifest.
var ErrBadSignature = errors.New("release signature does not match any trusted key")

// Verify checks sigText against message with the embedded trusted keys.
func Verify(message, sigText []byte) error {
	return VerifyWith(PublicKeys, message, sigText)
}

// VerifyWith checks sigText against message with the given base64 keys.
func VerifyWith(keys []string, message, sigText []byte) error {
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(sigText)))
	if err != nil || len(sig) != ed25519.SignatureSize {
		return fmt.Errorf("malformed release signature")
	}
	for _, k := range keys {
		pub, err := decodePublicKey(k)
		if err != nil {
			return err
		}
		if ed25519.Verify(pub, message, sig) {
			return nil
		}
	}
	return ErrBadSignature
}

// Sign returns the base64 signature of message using a base64 32-byte seed.
func Sign(seedB64 string, message []byte) (string, error) {
	seed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(seedB64))
	if err != nil || len(seed) != ed25519.SeedSize {
		return "", errors.New("signing key must be the base64 encoding of a 32-byte Ed25519 seed")
	}
	priv := ed25519.NewKeyFromSeed(seed)
	return base64.StdEncoding.EncodeToString(ed25519.Sign(priv, message)), nil
}

// PublicKeyFromSeed derives the base64 public key for a base64 seed.
func PublicKeyFromSeed(seedB64 string) (string, error) {
	seed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(seedB64))
	if err != nil || len(seed) != ed25519.SeedSize {
		return "", errors.New("invalid signing key")
	}
	pub := ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)
	return base64.StdEncoding.EncodeToString(pub), nil
}

// GenerateKey returns a new base64 seed (secret) and base64 public key.
func GenerateKey() (seedB64, pubB64 string, err error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}
	return base64.StdEncoding.EncodeToString(priv.Seed()), base64.StdEncoding.EncodeToString(pub), nil
}

// PEM returns the SubjectPublicKeyInfo PEM block for a base64 public key.
func PEM(pubB64 string) (string, error) {
	pub, err := decodePublicKey(pubB64)
	if err != nil {
		return "", err
	}
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return "", err
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})), nil
}

func decodePublicKey(k string) (ed25519.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(k))
	if err != nil || len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid embedded release public key %q", k)
	}
	return ed25519.PublicKey(raw), nil
}
