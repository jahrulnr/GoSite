# Domain Model

Entitas dan artefak file yang harus dipahami backend Go. **v1.3.1** mencakup registry plugin + ekstensi observability.

Ringkasan EN lengkap: [domain-model.md](./domain-model.md).

## Entitas database (SQLite)

### `users`, `websites`, `cronjobs`, `settings`

Sama seperti legacy BangunSite. `websites` ditambah kolom GoSite: `type` (`static`|`proxy`), `upstream`.

### `job_runs` (GoSite)

Worker certbot + cron manual — SSE output.

### `sessions` (GoSite)

Sesi panel persisten (`004_sessions.sql`): `id`, `user_id`, `expires_at`.

### Observability (`002_gosite_extensions.sql`)

`audit_logs`, `log_events`, `traffic_metrics`, `saved_queries` — dipakai Splunk Lite / Grafana Lite.

`nginx_status_samples`, `nginx_vts_server_samples`, `nginx_vts_upstream_samples` — metrik nginx real-time ([seq 22](../sequences/22-nginx-metrics_id.md)).

### `plugin_versions` (GoSite)

Satu baris per `(plugin_id, version)` — state machine install/enable (lihat [19-plugin-installer_id.md](./sequences/19-plugin-installer_id.md)).

Kolom provenance (007): `source_type`, `source_ref`, `resolved_url`, `install_log`, dll.

### `plugin_configs` (GoSite)

Config per versi + `secrets_encrypted`.

### Keyring plugin (filesystem)

`/storage/plugins/keyring.json` — bukan tabel SQL. API `/plugins/keyring`.

## Artefak filesystem

- Vhost nginx: `site.d/` + `active.d/`
- SSL: `/storage/webconfig/ssl/live/{domain}/`
- Log: `/storage/logs/access-{domain}.log`, …
- **Plugin:** `/storage/plugins/{plugin_id}/{version}/`

## Validasi bisnis

| Rule | Catatan |
|------|---------|
| Nginx | `nginx -t` + auto-repair sebelum reload |
| Plugin remote | Allowlist host, `permissions_ack`, signature policy |
| PHP/FPM | **Tidak di-port** ke GoSite |

## Environment variables relevan

| Var | Default | Pengaruh |
|-----|---------|----------|
| `AUTH_ENABLE` | false | Basic auth |
| `PLUGIN_REMOTE_INSTALL` | true | Matikan install remote |
| `PLUGIN_KEYRING_PATH` | `/storage/plugins/keyring.json` | Keyring |
| `GITHUB_TOKEN` / `GITLAB_TOKEN` | — | Repo privat |
| `TERMINAL_STICKY_TTL` | 12h | Reattach terminal |
| `DB_DATABASE` | `/storage/db.sqlite` | Path DB |
