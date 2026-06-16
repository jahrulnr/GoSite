package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// PluginConfig is the host-stored configuration for one plugin version.
type PluginConfig struct {
	PluginID         string
	Version          string
	ConfigVersion    string
	ConfigJSON       string
	SecretsEncrypted []byte
	UpdatedAt        time.Time
}

// PluginConfigRepository persists plugin configuration and secret blobs.
type PluginConfigRepository struct {
	db *sql.DB
}

// NewPluginConfigRepository returns a config repository backed by db.
func NewPluginConfigRepository(db *sql.DB) *PluginConfigRepository {
	return &PluginConfigRepository{db: db}
}

// ErrPluginConfigNotFound is returned when a config row does not exist.
var ErrPluginConfigNotFound = errors.New("plugin config not found")

// Get returns the configuration for (plugin_id, version). Returns
// ErrPluginConfigNotFound when the row does not exist.
func (r *PluginConfigRepository) Get(ctx context.Context, pluginID, version string) (PluginConfig, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT plugin_id, version, config_version, config_json, secrets_encrypted, updated_at
		FROM plugin_configs
		WHERE plugin_id = ? AND version = ?
	`, pluginID, version)
	var cfg PluginConfig
	var secrets sql.NullString
	if err := row.Scan(&cfg.PluginID, &cfg.Version, &cfg.ConfigVersion, &cfg.ConfigJSON, &secrets, &cfg.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PluginConfig{}, ErrPluginConfigNotFound
		}
		return PluginConfig{}, fmt.Errorf("scan plugin config: %w", err)
	}
	if secrets.Valid {
		cfg.SecretsEncrypted = []byte(secrets.String)
	}
	return cfg, nil
}

// Upsert stores or replaces the configuration row for (plugin_id, version).
// secretsEncrypted is stored as-is and is expected to already be encrypted
// by the caller.
func (r *PluginConfigRepository) Upsert(ctx context.Context, cfg PluginConfig) (PluginConfig, error) {
	now := time.Now().UTC()
	var secrets interface{}
	if len(cfg.SecretsEncrypted) > 0 {
		secrets = cfg.SecretsEncrypted
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO plugin_configs (plugin_id, version, config_version, config_json, secrets_encrypted, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(plugin_id, version) DO UPDATE SET
			config_version = excluded.config_version,
			config_json = excluded.config_json,
			secrets_encrypted = excluded.secrets_encrypted,
			updated_at = excluded.updated_at
	`, cfg.PluginID, cfg.Version, cfg.ConfigVersion, cfg.ConfigJSON, secrets, now)
	if err != nil {
		return PluginConfig{}, fmt.Errorf("upsert plugin config: %w", err)
	}
	cfg.UpdatedAt = now
	return cfg, nil
}

// Delete removes the configuration row for (plugin_id, version). It is a
// no-op when the row does not exist.
func (r *PluginConfigRepository) Delete(ctx context.Context, pluginID, version string) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM plugin_configs WHERE plugin_id = ? AND version = ?
	`, pluginID, version)
	if err != nil {
		return fmt.Errorf("delete plugin config: %w", err)
	}
	return nil
}
