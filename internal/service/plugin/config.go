package plugin

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/pkg/apperror"
	"github.com/jahrulnr/gosite/pkg/secrets"
)

// ConfigService stores, encrypts, and migrates per-version plugin
// configuration. It is intentionally separate from Service so lifecycle
// and configuration can evolve independently.
type ConfigService struct {
	repo          *sqlite.PluginConfigRepository
	migrator      ConfigMigrator
	cipher        *secrets.Cipher
	capabilitySet map[string]struct{}
}

// NewConfigService returns a config service bound to the given repository.
func NewConfigService(repo *sqlite.PluginConfigRepository, opts ...ConfigServiceOption) *ConfigService {
	svc := &ConfigService{
		repo:          repo,
		capabilitySet: map[string]struct{}{},
	}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

// ConfigServiceOption configures ConfigService.
type ConfigServiceOption func(*ConfigService)

// WithCipher supplies the secret cipher. When omitted the service is
// non-functional for secret storage and returns an error.
func WithCipher(c *secrets.Cipher) ConfigServiceOption {
	return func(s *ConfigService) { s.cipher = c }
}

// ConfigInput is the user-supplied config payload (form or API).
type ConfigInput struct {
	Version       string         `json:"version"`
	ConfigVersion string         `json:"configVersion,omitempty"`
	Values        map[string]any `json:"values"`
	Secrets       map[string]any `json:"secrets,omitempty"`
}

// View is the API response for a plugin config row. Secret fields are
// replaced with a nil placeholder so the wire never carries plaintext.
type View struct {
	PluginID      string         `json:"plugin_id"`
	Version       string         `json:"version"`
	ConfigVersion string         `json:"config_version"`
	Values        map[string]any `json:"values"`
	UpdatedAt     string         `json:"updated_at"`
}

// Get returns the config view for a plugin version. Secret fields are
// masked as nil.
func (s *ConfigService) Get(ctx context.Context, pluginID, version string) (View, error) {
	cfg, err := s.repo.Get(ctx, pluginID, version)
	if err != nil {
		if errors.Is(err, sqlite.ErrPluginConfigNotFound) {
			return View{}, apperror.New(apperror.CodeNotFound, "plugin config not found")
		}
		return View{}, apperror.Wrap(apperror.CodeDatabase, "get plugin config failed", err)
	}
	values, err := s.decodeValues(cfg.ConfigJSON)
	if err != nil {
		return View{}, apperror.Wrap(apperror.CodeConfig, "decode config failed", err)
	}
	return View{
		PluginID:      cfg.PluginID,
		Version:       cfg.Version,
		ConfigVersion: cfg.ConfigVersion,
		Values:        values,
		UpdatedAt:     cfg.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}, nil
}

// Put stores the supplied config and secret values. Returns the resulting
// view (secrets masked).
func (s *ConfigService) Put(ctx context.Context, pluginID string, in ConfigInput) (View, error) {
	pluginID = strings.TrimSpace(pluginID)
	version := strings.TrimSpace(in.Version)
	if pluginID == "" || version == "" {
		return View{}, apperror.New(apperror.CodeInvalidInput, "plugin_id and version are required")
	}
	if s.cipher == nil {
		return View{}, apperror.New(apperror.CodeConfig, "plugin secret cipher is not configured")
	}
	configJSON, err := json.Marshal(in.Values)
	if err != nil {
		return View{}, apperror.Wrap(apperror.CodeInvalidInput, "encode config failed", err)
	}
	var envelope []byte
	if len(in.Secrets) > 0 {
		secretsJSON, err := json.Marshal(in.Secrets)
		if err != nil {
			return View{}, apperror.Wrap(apperror.CodeInvalidInput, "encode secrets failed", err)
		}
		envelope, err = s.cipher.Encrypt(secretsJSON)
		if err != nil {
			return View{}, apperror.Wrap(apperror.CodeConfig, "encrypt secrets failed", err)
		}
	}
	saved, err := s.repo.Upsert(ctx, sqlite.PluginConfig{
		PluginID:         pluginID,
		Version:          version,
		ConfigVersion:    in.ConfigVersion,
		ConfigJSON:       string(configJSON),
		SecretsEncrypted: envelope,
	})
	if err != nil {
		return View{}, apperror.Wrap(apperror.CodeDatabase, "persist plugin config failed", err)
	}
	return s.Get(ctx, saved.PluginID, saved.Version)
}

// Delete removes a config row. Used by uninstall and purge flows.
func (s *ConfigService) Delete(ctx context.Context, pluginID, version string) error {
	if err := s.repo.Delete(ctx, pluginID, version); err != nil {
		return apperror.Wrap(apperror.CodeDatabase, "delete plugin config failed", err)
	}
	return nil
}

// RevealSecrets is used by the runtime layer to pass the decrypted secret
// blob to a plugin. Returns nil if the row has no secrets.
func (s *ConfigService) RevealSecrets(ctx context.Context, pluginID, version string) (map[string]any, error) {
	cfg, err := s.repo.Get(ctx, pluginID, version)
	if err != nil {
		if errors.Is(err, sqlite.ErrPluginConfigNotFound) {
			return nil, nil
		}
		return nil, apperror.Wrap(apperror.CodeDatabase, "load plugin config failed", err)
	}
	if len(cfg.SecretsEncrypted) == 0 {
		return nil, nil
	}
	if s.cipher == nil {
		return nil, apperror.New(apperror.CodeConfig, "plugin secret cipher is not configured")
	}
	plaintext, err := s.cipher.Decrypt(cfg.SecretsEncrypted)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeConfig, "decrypt secrets failed", err)
	}
	return s.decodeValues(string(plaintext))
}

func (s *ConfigService) decodeValues(raw string) (map[string]any, error) {
	if strings.TrimSpace(raw) == "" {
		return map[string]any{}, nil
	}
	var values map[string]any
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil, fmt.Errorf("decode values: %w", err)
	}
	return values, nil
}

// EnsureSchema is a small helper used by tests and bootstrap to keep the
// config table migrations aligned with the rest of the plugin stack.
func EnsureSchema(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS plugin_configs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		plugin_id TEXT NOT NULL,
		version TEXT NOT NULL,
		config_json TEXT NOT NULL DEFAULT '{}',
		secrets_encrypted BLOB,
		config_version TEXT NOT NULL DEFAULT '',
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE (plugin_id, version)
	)`)
	return err
}
