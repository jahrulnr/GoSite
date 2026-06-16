// Generate an Ed25519 keypair for plugin artifact signing.
//
// Usage:
//
//	go run keygen.go -out ~/.config/gosite/signing
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	out := flag.String("out", "signing", "output path prefix (writes .key and .pub.json)")
	keyID := flag.String("keyid", "", "optional key id for the public record")
	vendor := flag.String("vendor", "acme", "vendor slug for the public record")
	flag.Parse()

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fatal(err)
	}
	seed := priv.Seed()
	keyPath := *out + ".key"
	if err := os.WriteFile(keyPath, seed, 0o600); err != nil {
		fatal(err)
	}

	kid := *keyID
	if kid == "" {
		kid = *vendor + "-1"
	}
	record := map[string]string{
		"vendor":    *vendor,
		"keyId":     kid,
		"publicKey": base64.StdEncoding.EncodeToString(pub),
	}
	pubJSON, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		fatal(err)
	}
	pubPath := *out + ".pub.json"
	if err := os.WriteFile(pubPath, pubJSON, 0o644); err != nil {
		fatal(err)
	}

	fmt.Printf("wrote %s (private seed, keep secret)\n", filepath.Clean(keyPath))
	fmt.Printf("wrote %s (register with POST /api/v1/plugins/keyring)\n", filepath.Clean(pubPath))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
