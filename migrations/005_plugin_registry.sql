-- 005_plugin_registry.sql: plugin installer registry and lifecycle state machine

CREATE TABLE IF NOT EXISTS plugin_versions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    plugin_id TEXT NOT NULL,
    version TEXT NOT NULL,
    name TEXT NOT NULL,
    tier INTEGER NOT NULL,
    api_version TEXT NOT NULL,
    min_gosite_version TEXT NOT NULL DEFAULT '',
    rpc_version TEXT NOT NULL DEFAULT '',
    config_version TEXT NOT NULL DEFAULT '',
    manifest_json TEXT NOT NULL,
    capabilities_json TEXT NOT NULL DEFAULT '{}',
    ui_json TEXT NOT NULL DEFAULT '{}',
    artifact_digest TEXT NOT NULL,
    artifact_path TEXT NOT NULL,
    state TEXT NOT NULL CHECK (state IN (
        'installing',
        'installed',
        'install_failed',
        'enabling',
        'enabled',
        'enable_failed',
        'disabling',
        'uninstalling',
        'uninstalled'
    )),
    failure_class TEXT NOT NULL DEFAULT '',
    failure_message TEXT NOT NULL DEFAULT '',
    failure_at DATETIME,
    config_deleted_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (plugin_id, version)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_plugin_versions_one_enabled
    ON plugin_versions (plugin_id)
    WHERE state = 'enabled';

CREATE INDEX IF NOT EXISTS idx_plugin_versions_state
    ON plugin_versions (state);

CREATE INDEX IF NOT EXISTS idx_plugin_versions_cleanup
    ON plugin_versions (state, failure_class);
