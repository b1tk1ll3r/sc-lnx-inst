// Command release-sign manages the Ed25519 release signature of SHA256SUMS.txt.
//
//	release-sign keygen <private-key-file>  create a key pair; prints the public key
//	release-sign sign <file>                writes <file>.sig, key from $RELEASE_SIGNING_KEY
//	release-sign verify <file> [<sig>]      verifies with the embedded trusted keys
//	release-sign pem                        prints the embedded keys as PEM
package main

import (
	"errors"
	"fmt"
	"os"

	"citizen-launcher/backend/internal/signing"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "release-sign:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: release-sign keygen|sign|verify|pem ...")
	}
	switch args[0] {
	case "keygen":
		if len(args) != 2 {
			return errors.New("usage: release-sign keygen <private-key-file>")
		}
		seed, pub, err := signing.GenerateKey()
		if err != nil {
			return err
		}
		f, err := os.OpenFile(args[1], os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintln(f, seed); err != nil {
			_ = f.Close()
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
		fmt.Println(pub)
		return nil
	case "sign":
		if len(args) != 2 {
			return errors.New("usage: release-sign sign <file>")
		}
		seed := os.Getenv("RELEASE_SIGNING_KEY")
		if seed == "" {
			return errors.New("RELEASE_SIGNING_KEY is not set")
		}
		pub, err := signing.PublicKeyFromSeed(seed)
		if err != nil {
			return err
		}
		if !trusted(pub) {
			return errors.New("RELEASE_SIGNING_KEY does not belong to an embedded trusted public key")
		}
		msg, err := os.ReadFile(args[1])
		if err != nil {
			return err
		}
		sig, err := signing.Sign(seed, msg)
		if err != nil {
			return err
		}
		return os.WriteFile(args[1]+".sig", []byte(sig+"\n"), 0o644)
	case "verify":
		if len(args) != 2 && len(args) != 3 {
			return errors.New("usage: release-sign verify <file> [<sig>]")
		}
		sigPath := args[1] + ".sig"
		if len(args) == 3 {
			sigPath = args[2]
		}
		msg, err := os.ReadFile(args[1])
		if err != nil {
			return err
		}
		sig, err := os.ReadFile(sigPath)
		if err != nil {
			return err
		}
		if err := signing.Verify(msg, sig); err != nil {
			return err
		}
		fmt.Println("signature OK")
		return nil
	case "pem":
		for _, k := range signing.PublicKeys {
			p, err := signing.PEM(k)
			if err != nil {
				return err
			}
			fmt.Print(p)
		}
		return nil
	}
	return fmt.Errorf("unknown command %q", args[0])
}

func trusted(pub string) bool {
	for _, k := range signing.PublicKeys {
		if k == pub {
			return true
		}
	}
	return false
}
