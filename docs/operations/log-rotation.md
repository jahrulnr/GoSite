# Log Rotation & SQLite Maintenance

**Status:** Implemented

GoSite runs inside a single Docker container with nginx and `gosite serve` as parallel processes. Without active log rotation, raw nginx `.log` files under `/storage/logs/` grow indefinitely. Similarly, the SQLite database (`/storage/db.sqlite`) can bloat after retention purges leave free pages behind.

## Log rotation (raw nginx logs)

### Config

| File | Role |
|------|------|
| `config/logrotate/gosite` | logrotate config (shipped into image at `/etc/logrotate.d/gosite`) |
| `Dockerfile` | Installs `logrotate` package |
| `config/start.sh` | Background loop: `logrotate /etc/logrotate.d/gosite` every 24h |

### Policy

```
/storage/logs/*.log {
    daily
    rotate 14
    maxage 14
    compress
    delaycompress
    missingok
    notifempty
    copytruncate
    dateext
    dateformat -%Y%m%d
}
```

- **Daily** rotation
- **14 rotations** kept (2 weeks max)
- **compress** + **delaycompress** (gzip previous rotation, not the current one)
- **copytruncate** (no nginx reload needed — copies then truncates the active file)
- **dateext** — rotated files named `access.log-20260711.gz` etc.

### How it runs

`start.sh` spawns a background subshell before `exec gosite serve`:

```bash
(
  sleep 60   # wait for nginx to open log files
  while true; do
    logrotate /etc/logrotate.d/gosite 2>/dev/null || true
    sleep 86400
  done
) &
```

No cron daemon required — the loop is self-contained.

## SQLite maintenance (VACUUM)

### Problem

The retention purge loop (`internal/app/app.go` → `runRetentionPurge`) deletes expired rows from `audit_logs`, `log_events`, `traffic_metrics`, `nginx_status_samples`, and `nginx_vts_*_samples` every 24h. SQLite marks deleted pages as free but does **not** shrink the database file — free pages are reused for future inserts but the file size never decreases.

### Solution

After all retention purges complete, `runRetentionPurge` calls `sqlite.Vacuum(db)` to run `VACUUM`, which rebuilds the database file and reclaims free space.

| File | Role |
|------|------|
| `internal/repository/sqlite/db.go` → `Vacuum()` | Executes `VACUUM` on the database |
| `internal/app/app.go` → `runRetentionPurge` | Calls `Vacuum` after all purges |

### Retention defaults

| Table | Env var | Default |
|-------|---------|---------|
| `audit_logs` | `AUDIT_RETENTION_DAYS` | 90 days |
| `log_events` | `LOG_EVENTS_RETENTION_DAYS` | 14 days |
| `traffic_metrics` | `LOG_EVENTS_RETENTION_DAYS` | 14 days |
| `nginx_status_samples` | `LOG_EVENTS_RETENTION_DAYS` | 14 days |
| `nginx_vts_*_samples` | `LOG_EVENTS_RETENTION_DAYS` | 14 days |

### Schedule

Both log rotation and retention purge + VACUUM run on a **24-hour cycle**. The first VACUUM runs 24h after container start.

## Code

| File | Role |
|------|------|
| `config/logrotate/gosite` | logrotate policy |
| `config/start.sh` | Background logrotate loop |
| `Dockerfile` | Installs `logrotate`, copies config |
| `internal/repository/sqlite/db.go` | `Vacuum()` function |
| `internal/app/app.go` | `runRetentionPurge` → purge + VACUUM |
