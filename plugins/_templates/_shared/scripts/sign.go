// Sign a plugin zip: embed artifact.sha256 + signatures in manifest, re-pack.
//
// Usage:
//
//	go run sign.go -artifact dist/foo.zip -manifest manifest.json -key signing.key -keyid acme-1 -vendor acme
package main

import (
	"archive/zip"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	artifact := flag.String("artifact", "", "path to plugin zip")
	manifestPath := flag.String("manifest", "manifest.json", "source manifest json")
	keyPath := flag.String("key", "", "ed25519 seed file (32 bytes)")
	keyID := flag.String("keyid", "vendor-1", "signature key id")
	vendor := flag.String("vendor", "", "vendor slug for manifest id prefix")
	flag.Parse()

	if *artifact == "" || *keyPath == "" {
		flag.Usage()
		os.Exit(1)
	}

	zipBytes, err := os.ReadFile(*artifact)
	if err != nil {
		fatal(err)
	}
	sum := sha256.Sum256(zipBytes)
	digest := hex.EncodeToString(sum[:])

	seed, err := os.ReadFile(*keyPath)
	if err != nil {
		fatal(err)
	}
	if len(seed) != ed25519.SeedSize {
		fatal(fmt.Errorf("key must be %d bytes (ed25519 seed)", ed25519.SeedSize))
	}
	priv := ed25519.NewKeyFromSeed(seed)
	sig := ed25519.Sign(priv, []byte(digest))

	var manifest map[string]any
	data, err := os.ReadFile(*manifestPath)
	if err != nil {
		fatal(err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		fatal(err)
	}
	if *vendor == "" {
		if id, ok := manifest["id"].(string); ok {
			*vendor = strings.SplitN(id, "/", 2)[0]
		}
	}
	manifest["signatures"] = []map[string]string{{
		"keyId": *keyID,
		"sig":   base64.StdEncoding.EncodeToString(sig),
	}}
	// artifact.sha256 is optional; host verifies signature over the uploaded zip digest.

	outDir := filepath.Dir(*artifact)
	if err := repackZip(outDir, *artifact, manifest); err != nil {
		fatal(err)
	}

	signedBytes, err := os.ReadFile(*artifact)
	if err != nil {
		fatal(err)
	}
	uploadSum := sha256.Sum256(signedBytes)
	uploadDigest := hex.EncodeToString(uploadSum[:])

	meta := map[string]string{
		"signedDigest": digest,
		"uploadDigest": uploadDigest,
		"keyId":        *keyID,
	}
	metaJSON, _ := json.MarshalIndent(meta, "", "  ")
	metaPath := *artifact + ".sigmeta"
	if err := os.WriteFile(metaPath, metaJSON, 0o644); err != nil {
		fatal(err)
	}

	fmt.Printf("signed %s signedDigest=%s uploadDigest=%s keyId=%s\n", *artifact, digest, uploadDigest, *keyID)
	fmt.Printf("sigmeta %s\n", metaPath)
}

func repackZip(dir, artifact string, manifest map[string]any) error {
	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	r, err := zip.OpenReader(artifact)
	if err != nil {
		return err
	}
	defer r.Close()

	tmp := artifact + ".unsigned"
	_ = os.Remove(tmp)
	w, err := os.Create(tmp)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(w)
	if err := writeFile(zw, "manifest.json", manifestJSON); err != nil {
		zw.Close()
		w.Close()
		return err
	}
	for _, f := range r.File {
		if f.Name == "manifest.json" {
			continue
		}
		if err := copyZipEntry(zw, f); err != nil {
			zw.Close()
			w.Close()
			return err
		}
	}
	if err := zw.Close(); err != nil {
		w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, artifact)
}

func writeFile(zw *zip.Writer, name string, data []byte) error {
	h := &zip.FileHeader{Name: name, Method: zip.Deflate}
	w, err := zw.CreateHeader(h)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func copyZipEntry(zw *zip.Writer, f *zip.File) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	h := &zip.FileHeader{Name: f.Name, Method: f.Method}
	w, err := zw.CreateHeader(h)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, rc)
	return err
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
