// Package secrets provides envelope encryption for plugin secret blobs.
//
// The key is sourced from PLUGIN_CONFIG_KEY (base64-encoded 32 bytes) and
// falls back to a derived key from the storage directory when unset. The
// helper is intentionally minimal: encrypt the secret blob as AES-GCM and
// expose the result as a binary envelope. Rotation and KMS integration are
// out of scope and can be added later by wrapping the Source interface.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
)

// Source produces the symmetric key used for encryption. Implementations
// are responsible for key rotation and KMS integration.
type Source interface {
	Key() ([]byte, error)
}

// EnvSource reads the key from the PLUGIN_CONFIG_KEY environment variable.
// The value must be a base64-encoded 32-byte (AES-256) key.
type EnvSource struct{}

// NewEnvSource returns a Source that reads PLUGIN_CONFIG_KEY.
func NewEnvSource() EnvSource { return EnvSource{} }

// Key returns the 32-byte AES key.
func (EnvSource) Key() ([]byte, error) {
	raw := os.Getenv("PLUGIN_CONFIG_KEY")
	if raw == "" {
		return nil, errors.New("PLUGIN_CONFIG_KEY is not set")
	}
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("PLUGIN_CONFIG_KEY is not base64: %w", err)
	}
	if len(decoded) != 32 {
		return nil, fmt.Errorf("PLUGIN_CONFIG_KEY must decode to 32 bytes, got %d", len(decoded))
	}
	return decoded, nil
}

// DerivedSource hashes a per-host seed (typically a path) to a 32-byte key.
// It is a development fallback only and not a substitute for a real KMS.
type DerivedSource struct {
	Seed string
}

// NewDerivedSource returns a Source keyed off seed.
func NewDerivedSource(seed string) DerivedSource { return DerivedSource{Seed: seed} }

// Key returns the derived 32-byte key.
func (s DerivedSource) Key() ([]byte, error) {
	if s.Seed == "" {
		return nil, errors.New("derived source requires a non-empty seed")
	}
	sum := sha256.Sum256([]byte("gosite-plugin-config:" + s.Seed))
	return sum[:], nil
}

// Cipher is the high-level encrypt/decrypt helper.
type Cipher struct {
	source Source

	mu     sync.RWMutex
	key    []byte
	gcm    cipher.AEAD
}

// NewCipher returns a Cipher using the given source. The key is loaded
// eagerly so that misconfiguration is detected at startup, not at first
// encrypt call.
func NewCipher(source Source) (*Cipher, error) {
	if source == nil {
		return nil, errors.New("nil secret source")
	}
	key, err := source.Key()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("init aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("init gcm: %w", err)
	}
	return &Cipher{source: source, key: key, gcm: gcm}, nil
}

// Encrypt seals plaintext under the configured key. The output is a
// self-describing envelope: [version:1][nonce:12][ciphertext+N].
func (c *Cipher) Encrypt(plaintext []byte) ([]byte, error) {
	c.mu.RLock()
	gcm := c.gcm
	c.mu.RUnlock()
	if gcm == nil {
		return nil, errors.New("cipher not initialised")
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("nonce: %w", err)
	}
	sealed := gcm.Seal(nil, nonce, plaintext, nil)
	out := make([]byte, 0, 1+len(nonce)+len(sealed))
	out = append(out, 0x01)
	out = append(out, nonce...)
	out = append(out, sealed...)
	return out, nil
}

// Decrypt opens an envelope produced by Encrypt.
func (c *Cipher) Decrypt(envelope []byte) ([]byte, error) {
	c.mu.RLock()
	gcm := c.gcm
	c.mu.RUnlock()
	if gcm == nil {
		return nil, errors.New("cipher not initialised")
	}
	if len(envelope) < 1+gcm.NonceSize() {
		return nil, errors.New("secret envelope too short")
	}
	if envelope[0] != 0x01 {
		return nil, fmt.Errorf("unknown secret envelope version %d", envelope[0])
	}
	nonce := envelope[1 : 1+gcm.NonceSize()]
	sealed := envelope[1+gcm.NonceSize():]
	return gcm.Open(nil, nonce, sealed, nil)
}
