-- 009_nginx_status_samples.sql: stub_status time-series samples

CREATE TABLE IF NOT EXISTS nginx_status_samples (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    sample_ts DATETIME NOT NULL,
    active INTEGER NOT NULL DEFAULT 0,
    accepts INTEGER NOT NULL DEFAULT 0,
    handled INTEGER NOT NULL DEFAULT 0,
    requests INTEGER NOT NULL DEFAULT 0,
    reading INTEGER NOT NULL DEFAULT 0,
    writing INTEGER NOT NULL DEFAULT 0,
    waiting INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_nginx_status_samples_ts ON nginx_status_samples (sample_ts);
