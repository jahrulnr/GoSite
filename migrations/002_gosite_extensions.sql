-- 002_gosite_extensions.sql: GoSite-specific schema extensions

ALTER TABLE websites ADD COLUMN type TEXT NOT NULL DEFAULT 'static';
ALTER TABLE websites ADD COLUMN upstream TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS audit_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    ts DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    user_email TEXT NOT NULL DEFAULT '',
    action TEXT NOT NULL,
    resource_type TEXT NOT NULL DEFAULT '',
    resource_id TEXT NOT NULL DEFAULT '',
    domain TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK (status IN ('ok', 'failed')),
    message TEXT NOT NULL DEFAULT '',
    meta_json TEXT NOT NULL DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS job_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_type TEXT NOT NULL,
    name TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending', 'running', 'ok', 'failed', 'cancelled')),
    output TEXT NOT NULL DEFAULT '',
    error TEXT NOT NULL DEFAULT '',
    started_at DATETIME,
    finished_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS log_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    ts DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    source TEXT NOT NULL CHECK (source IN ('access', 'error')),
    site TEXT NOT NULL DEFAULT '',
    status_code INTEGER,
    bytes INTEGER,
    line_hash TEXT NOT NULL DEFAULT '',
    raw_preview TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS traffic_metrics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    bucket_ts DATETIME NOT NULL,
    site TEXT NOT NULL DEFAULT '',
    requests INTEGER NOT NULL DEFAULT 0,
    bytes INTEGER NOT NULL DEFAULT 0,
    s2xx INTEGER NOT NULL DEFAULT 0,
    s3xx INTEGER NOT NULL DEFAULT 0,
    s4xx INTEGER NOT NULL DEFAULT 0,
    s5xx INTEGER NOT NULL DEFAULT 0,
    UNIQUE (bucket_ts, site)
);

CREATE TABLE IF NOT EXISTS saved_queries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    source TEXT NOT NULL,
    query TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_ts ON audit_logs (ts);
CREATE INDEX IF NOT EXISTS idx_job_runs_created_at ON job_runs (created_at);
CREATE INDEX IF NOT EXISTS idx_log_events_ts ON log_events (ts);
CREATE INDEX IF NOT EXISTS idx_traffic_metrics_bucket_site ON traffic_metrics (bucket_ts, site);
