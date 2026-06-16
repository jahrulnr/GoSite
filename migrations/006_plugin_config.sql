-- 006_plugin_config.sql: plugin configuration storage with encrypted secrets

CREATE TABLE IF NOT EXISTS plugin_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    plugin_id TEXT NOT NULL,
    version TEXT NOT NULL,
    config_json TEXT NOT NULL DEFAULT '{}',
    secrets_encrypted BLOB,
    config_version TEXT NOT NULL DEFAULT '',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (plugin_id, version)
);

CREATE INDEX IF NOT EXISTS idx_plugin_configs_lookup
    ON plugin_configs (plugin_id, version);
