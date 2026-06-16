// Verify a signed plugin zip against a public key record from keygen.go.
//
// Usage:
//
//	go run verify.go -artifact dist/foo.zip -pub signing.pub.json
package main

import (
	"archive/zip"
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
)

func main() {
	artifact := flag.String("artifact", "", "path to plugin zip")
	pubPath := flag.String("pub", "", "path to .pub.json from keygen.go")
	sigmeta := flag.String("sigmeta", "", "optional .sigmeta from sign.go (default: <artifact>.sigmeta)")
	flag.Parse()

	if *artifact == "" || *pubPath == "" {
		flag.Usage()
		os.Exit(1)
	}

	metaPath := *sigmeta
	if metaPath == "" {
		metaPath = *artifact + ".sigmeta"
	}

	var record struct {
		KeyID     string `json:"keyId"`
		PublicKey string `json:"publicKey"`
	}
	data, err := os.ReadFile(*pubPath)
	if err != nil {
		fatal(err)
	}
	if err := json.Unmarshal(data, &record); err != nil {
		fatal(err)
	}
	pubBytes, err := base64.StdEncoding.DecodeString(record.PublicKey)
	if err != nil {
		fatal(err)
	}
	if len(pubBytes) != ed25519.PublicKeySize {
		fatal(fmt.Errorf("public key must be %d bytes", ed25519.PublicKeySize))
	}

	zipBytes, err := os.ReadFile(*artifact)
	if err != nil {
		fatal(err)
	}
	sum := sha256.Sum256(zipBytes)
	uploadDigest := hex.EncodeToString(sum[:])

	signedDigest := uploadDigest
	if metaBytes, err := os.ReadFile(metaPath); err == nil {
		var meta struct {
			SignedDigest string `json:"signedDigest"`
		}
		if json.Unmarshal(metaBytes, &meta) == nil && meta.SignedDigest != "" {
			signedDigest = meta.SignedDigest
		}
	}

	manifest, err := readManifestFromZip(zipBytes)
	if err != nil {
		fatal(err)
	}
	artifactBlock, _ := manifest["artifact"].(map[string]any)
	embedded, _ := artifactBlock["sha256"].(string)
	if embedded != "" && embedded != uploadDigest {
		fatal(fmt.Errorf("embedded sha256 mismatch: manifest=%s upload=%s", embedded, uploadDigest))
	}

	sigs, _ := manifest["signatures"].([]any)
	if len(sigs) == 0 {
		fatal(fmt.Errorf("no signatures in manifest"))
	}
	sigBlock, _ := sigs[0].(map[string]any)
	sigB64, _ := sigBlock["sig"].(string)
	sigBytes, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		fatal(err)
	}
	if !ed25519.Verify(ed25519.PublicKey(pubBytes), []byte(signedDigest), sigBytes) {
		fatal(fmt.Errorf("signature invalid for signedDigest %s (uploadDigest %s)", signedDigest, uploadDigest))
	}
	fmt.Printf("OK artifact=%s keyId=%s signedDigest=%s uploadDigest=%s\n", *artifact, record.KeyID, signedDigest, uploadDigest)
}

func readManifestFromZip(zipBytes []byte) (map[string]any, error) {
	r, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return nil, err
	}
	for _, f := range r.File {
		if f.Name != "manifest.json" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		raw, err := io.ReadAll(rc)
		if err != nil {
			return nil, err
		}
		var manifest map[string]any
		if err := json.Unmarshal(raw, &manifest); err != nil {
			return nil, err
		}
		return manifest, nil
	}
	return nil, fmt.Errorf("manifest.json not found in zip")
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
