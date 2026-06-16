package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// KeyringEntry is a single trusted-key record used by the keyring admin
// API and the install signature check.
type KeyringEntry struct {
	Vendor    string `json:"vendor"`
	KeyID     string `json:"keyId"`
	PublicKey string `json:"publicKey"`
	CreatedAt string `json:"createdAt,omitempty"`
	RevokedAt string `json:"revokedAt,omitempty"`
}

// LoadKeyring reads the keyring JSON file from disk. Returns an empty
// keyring when the file does not exist yet (treat as "no trusted keys").
func LoadKeyring(_ context.Context, path string) ([]KeyringEntry, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("keyring path is empty")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []KeyringEntry{}, nil
		}
		return nil, fmt.Errorf("read keyring: %w", err)
	}
	if strings.TrimSpace(string(data)) == "" {
		return []KeyringEntry{}, nil
	}
	var keyring Keyring
	if err := json.Unmarshal(data, &keyring); err != nil {
		return nil, fmt.Errorf("parse keyring: %w", err)
	}
	out := make([]KeyringEntry, 0, len(keyring.Keys))
	for _, k := range keyring.Keys {
		out = append(out, KeyringEntry{
			Vendor:    k.Vendor,
			KeyID:     k.KeyID,
			PublicKey: k.PublicKey,
			RevokedAt: k.RevokedAt,
		})
	}
	return out, nil
}

// AddKeyringEntry appends (or replaces) a key in the keyring file. New
// keys are stamped with the current time in RFC3339 UTC.
func AddKeyringEntry(_ context.Context, path string, key TrustedKey) error {
	if strings.TrimSpace(key.Vendor) == "" || strings.TrimSpace(key.KeyID) == "" || strings.TrimSpace(key.PublicKey) == "" {
		return fmt.Errorf("vendor, keyId, and publicKey are required")
	}
	if err := os.MkdirAll(parentDir(path), 0o755); err != nil {
		return err
	}
	kr, _ := loadKeyringFile(path)
	replaced := false
	for i := range kr.Keys {
		if kr.Keys[i].Vendor == key.Vendor && kr.Keys[i].KeyID == key.KeyID {
			kr.Keys[i] = key
			replaced = true
			break
		}
	}
	if !replaced {
		kr.Keys = append(kr.Keys, key)
	}
	return writeKeyringFile(path, kr)
}

// RevokeKeyringEntry marks a key as revoked by stamping revokedAt.
func RevokeKeyringEntry(_ context.Context, path, vendor, keyID string) error {
	if strings.TrimSpace(vendor) == "" || strings.TrimSpace(keyID) == "" {
		return fmt.Errorf("vendor and keyId required")
	}
	kr, err := loadKeyringFile(path)
	if err != nil {
		return err
	}
	for i := range kr.Keys {
		if kr.Keys[i].Vendor == vendor && kr.Keys[i].KeyID == keyID {
			if kr.Keys[i].RevokedAt == "" {
				kr.Keys[i].RevokedAt = time.Now().UTC().Format(time.RFC3339)
			}
			return writeKeyringFile(path, kr)
		}
	}
	return fmt.Errorf("key not found")
}

func loadKeyringFile(path string) (Keyring, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Keyring{}, nil
		}
		return Keyring{}, err
	}
	var kr Keyring
	if len(data) > 0 {
		if err := json.Unmarshal(data, &kr); err != nil {
			return Keyring{}, err
		}
	}
	return kr, nil
}

func writeKeyringFile(path string, kr Keyring) error {
	data, err := json.MarshalIndent(kr, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func parentDir(path string) string {
	idx := strings.LastIndexAny(path, "/\\")
	if idx < 0 {
		return "."
	}
	return path[:idx]
}
