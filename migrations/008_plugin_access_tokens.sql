-- 008_plugin_access_tokens.sql: scoped integration tokens for MCP and tier-0 webhooks

CREATE TABLE IF NOT EXISTS plugin_access_tokens (
    id TEXT PRIMARY KEY,
    plugin_id TEXT NOT NULL,
    created_under_version TEXT NOT NULL DEFAULT '',
    label TEXT NOT NULL DEFAULT '',
    token_hash TEXT NOT NULL,
    scopes_json TEXT NOT NULL DEFAULT '[]',
    created_by_user_id INTEGER NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME,
    revoked_at DATETIME,
    last_used_at DATETIME
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_plugin_access_tokens_hash
    ON plugin_access_tokens (token_hash);

CREATE INDEX IF NOT EXISTS idx_plugin_access_tokens_plugin
    ON plugin_access_tokens (plugin_id);

CREATE INDEX IF NOT EXISTS idx_plugin_access_tokens_active
    ON plugin_access_tokens (plugin_id, revoked_at);
