> **English:** [API-reference](API-reference)

REST API GoSite v1.

Kontrak: [api/openapi.yaml](https://github.com/jahrulnr/GoSite/blob/master/api/openapi.yaml) · [api/examples/](https://github.com/jahrulnr/GoSite/blob/master/api/examples/)
```json
{ "error": { "code": "VALIDATION_FAILED", "message": "human-readable message" } }
```

## Konvensi

- Base URL panel: `https://host:8080/api/v1`
- Auth: `POST /auth/login` → token/session cookie
- Semua endpoint di bawah `/admin/*` legacy → butuh auth
- Endpoint `/api/server/*` legacy **tanpa auth** — di GoSite **wajib** dilindungi auth

---

## Auth

| Legacy | Method | Usulan GoSite |
|--------|--------|---------------|
| `GET /` | GET | `GET /auth/login` (metadata: lockscreen enabled, basic auth) |
| `POST /` | POST | `POST /auth/login` `{ email, password, remember }` |
| `GET /locked` | GET | `GET /auth/lockscreen` |
| BasicAuth middleware | — | `401` + `WWW-Authenticate` jika `AUTH_ENABLE=true` |

**Response login sukses:**
```json
{ "token": "...", "user": { "id": 1, "name": "Admin", "email": "admin@demo.com" } }
```

---

## Dashboard & monitoring

| Legacy | Method | Usulan GoSite |
|--------|--------|---------------|
| `GET /admin/` | GET | `GET /dashboard` — snapshot awal |
| `POST /api/server/info` | POST | `GET /system/info` |
| `POST /api/server/traffic` | POST | `GET /system/network` |
| `POST /api/server/diskIO` | POST | `GET /system/disk-io` |
| `POST /api/server/nginx/traffic` | POST | `GET /system/nginx-traffic` |

---

## Website

| Legacy | Method | Usulan GoSite |
|--------|--------|---------------|
| `GET /admin/website` | GET | `GET /websites` |
| `POST /admin/website` | POST | `POST /websites` |
| `GET /admin/website/{id}/edit` | GET | `GET /websites/{id}` |
| `PUT/PATCH /admin/website/{id}` | PUT | `PUT /websites/{id}` |
| `OPTIONS /admin/website/{id}/enableSite` | PATCH | `PATCH /websites/{id}/toggle` |
| `POST /admin/website/{id}/updateConfig` | POST | `PUT /websites/{id}/nginx-config` |
**Validasi create/update** — `POST /websites/validate`

```json
{ "domain", "path", "type", "upstream?", "active", "id?" }
→ { "valid": true } | { "valid": false, "reason": "..." }
```

- Memvalidasi domain, path, upstream (proxy), duplikasi path
- Jika `active: true`, menjalankan `nginx -t` terisolasi pada config **rendered** (file temp, **tidak** menulis `site.d`)

| Legacy | Method | GoSite |
|--------|--------|--------|
| `GET /admin/website/{id}/installSSL` | GET | `POST /websites/{id}/ssl/certbot` → `202 { job_id }` |
| — | GET | `GET /websites/{id}/ssl/certbot/stream?job_id=` (SSE) |
| `POST /admin/website/{id}/updateSSL` | POST | `PUT /websites/{id}/ssl/manual` |
| `DELETE /admin/website/{id}/enableSite` | DELETE | `DELETE /websites/{id}?clean=true` |
| `PATCH /admin/website/updateNginx` | PATCH | `PUT /nginx/global` |
| — | GET | `GET /nginx/default` — baca default.conf |
| — | PUT | `PUT /nginx/default` |

**Validasi create/update** (endpoint terpisah atau inline):
`POST /websites/validate` — lihat detail di atas.

---

## CLI (boot & ops)

| Perintah | Keterangan |
|----------|------------|
| `gosite init` | First-boot storage + migrate + seed |
| `gosite migrate` | Apply migrations |
| `gosite serve` | HTTP API + SPA |
| `gosite nginx-repair` | `nginx -t` + auto-fix aman ([nginx-repair.md](Nginx-auto-repair-id)) |

Dipanggil dari `config/start.sh` sebelum nginx + gosite serve.

---

## Docker

| Legacy | Method | Usulan GoSite |
|--------|--------|---------------|
| `GET /admin/docker` | GET | `GET /docker/containers` |
| `GET /admin/docker/restart/{id}` | GET | `POST /docker/containers/{id}/restart` |
| `GET /admin/docker/stop/{id}` | GET | `POST /docker/containers/{id}/stop` |
| `GET /admin/docker/log/{id}` | GET | `GET /docker/containers/{id}/logs?tail=200` |

---

## File manager

| Legacy | Method | Usulan GoSite |
|--------|--------|---------------|
| `GET /admin/browse?path=` | GET | `GET /files?path=/www` |
| `POST /admin/browse/show` | POST | `GET /files/content?path=...` |
| `POST /admin/browse/new` | POST | `POST /files` — type: directory\|file\|remote\|upload |
| `PATCH /admin/browse/action` | PATCH | `POST /files/actions` — chmod\|copy\|execute |
| `DELETE /admin/browse/action` | DELETE | `DELETE /files?path=...` |

---

## Mount manager

| Legacy | Method | Usulan GoSite |
|--------|--------|---------------|
| `GET /admin/mount` | GET | `GET /mounts` |
| `POST /admin/mount` | POST | `POST /mounts` |
| `POST /admin/mount/update` | POST | `PUT /mounts` (identify by device+dir) |
| `GET /admin/mount/enable` | GET | `POST /mounts/enable` |
| `GET /admin/mount/delete` | GET | `DELETE /mounts` |

---

## Cron jobs

| Legacy | Method | Usulan GoSite |
|--------|--------|---------------|
| `GET /admin/cronjob` | GET | `GET /cronjobs` |
| `POST /admin/cronjob` | POST | `POST /cronjobs` |
| `PUT /admin/cronjob/{id}` | PUT | `PUT /cronjobs/{id}` |
| `DELETE /admin/cronjob/{id}` | DELETE | `DELETE /cronjobs/{id}` |
| `POST /admin/cronjob/run/{id}` | POST | `POST /cronjobs/{id}/run` + stream output |

---

## Settings

| Legacy | Method | Usulan GoSite |
|--------|--------|---------------|
| `GET /admin/setting` | GET | `GET /settings` |
| `POST /admin/setting/update/profile` | POST | `PUT /settings/profile` |
| `POST /admin/setting/update/php` | POST | `PUT /settings/php` |
| `POST /admin/setting/update/fpm` | POST | `PUT /settings/fpm` |
| `POST /admin/setting/update/pool` | POST | `PUT /settings/pool` |

---

## Logs

| Legacy | Method | Usulan GoSite |
|--------|--------|---------------|
| `GET /admin/logs` | GET | `GET /logs/sites` |
| `GET /admin/logs/get` | GET | `GET /logs?domain=&type=access\|error&tail=1000` |

---

## Database viewer

| Legacy | Method | Usulan GoSite |
|--------|--------|---------------|
| `GET /admin/database` | GET | `GET /database/tables` |
| `GET /admin/database/{col}` | GET | `GET /database/tables/{name}?limit=100` |

---

## Health

| Legacy | Method | GoSite (implemented) |
|--------|--------|----------------------|
| `ANY /healty` | GET | `GET /health` |

---

## Splunk Lite (seq 17)

| Method | Path | Body / query |
|--------|------|----------------|
| POST | `/api/v1/query` | `{ "source": "audit\|job\|access\|error\|all", "q": "action:vhost.*", "from": "-24h", "to": "now", "limit": 100 }` |
| GET | `/api/v1/query/saved` | — |
| POST | `/api/v1/query/saved` | `{ "name", "source", "query" }` |

---

## Grafana Lite (seq 18)

| Method | Path | Query |
|--------|------|-------|
| GET | `/api/v1/metrics/traffic/series` | `range=24h&step=5m&site=` |
| GET | `/api/v1/metrics/traffic/top-sites` | `range=1h&limit=10` |
| GET | `/api/v1/metrics/traffic/status-codes` | `range=24h&site=` |
| GET | `/api/v1/metrics/traffic/summary` | `range=1h` |

---

## Nginx ops

| Method | Path | Keterangan |
|--------|------|------------|
| POST | `/api/v1/nginx/reload` | `TestAndRepair` lalu `nginx -s reload` |
| POST | `/api/v1/nginx/test` | Test raw config body |
| GET/PUT | `/api/v1/nginx/global` | `nginx.conf` |
| GET/PUT | `/api/v1/nginx/default` | `http.d/default.conf` |

Setiap reload internal memanggil [nginx auto-repair](Nginx-auto-repair-id) terlebih dahulu.

---

## Error format (implemented)

```json
{ "error": { "code": "NGINX_TEST_FAILED", "message": "..." } }
```

---

## Endpoint yang butuh streaming / long-poll

Legacy memakai polling file `/tmp/*.txt` untuk output async:

| Fitur | GoSite |
|-------|--------|
| Certbot install | SSE `GET /websites/{id}/ssl/certbot/stream?job_id=` — event `done` saat selesai |
| Cron manual run | SSE `GET /cronjobs/{id}/run/stream` |
| Docker logs | Optional SSE untuk follow mode |

---

## Catatan keamanan migrasi

1. Ubah semua `GET` yang menjalankan aksi (docker restart/stop) → `POST`
2. Lindungi `/system/*` dengan auth (legacy terbuka)
3. File manager & mount: validasi path ketat, deny di luar allowlist root
4. Cron payload: pertimbangkan allowlist atau approval untuk command berbahaya
