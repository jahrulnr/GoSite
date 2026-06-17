-- 010_nginx_vts_samples.sql: VTS server/upstream snapshots

CREATE TABLE IF NOT EXISTS nginx_vts_server_samples (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    sample_ts DATETIME NOT NULL,
    server_name TEXT NOT NULL,
    requests INTEGER NOT NULL DEFAULT 0,
    in_bytes INTEGER NOT NULL DEFAULT 0,
    out_bytes INTEGER NOT NULL DEFAULT 0,
    request_msec REAL NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS nginx_vts_upstream_samples (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    sample_ts DATETIME NOT NULL,
    upstream_name TEXT NOT NULL,
    server_addr TEXT NOT NULL DEFAULT '',
    requests INTEGER NOT NULL DEFAULT 0,
    in_bytes INTEGER NOT NULL DEFAULT 0,
    out_bytes INTEGER NOT NULL DEFAULT 0,
    response_msec REAL NOT NULL DEFAULT 0,
    down INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_nginx_vts_server_samples_ts ON nginx_vts_server_samples (sample_ts);
CREATE INDEX IF NOT EXISTS idx_nginx_vts_upstream_samples_ts ON nginx_vts_upstream_samples (sample_ts);
CREATE INDEX IF NOT EXISTS idx_nginx_vts_server_samples_name_ts ON nginx_vts_server_samples (server_name, sample_ts);
CREATE INDEX IF NOT EXISTS idx_nginx_vts_upstream_samples_name_ts ON nginx_vts_upstream_samples (upstream_name, server_addr, sample_ts);
