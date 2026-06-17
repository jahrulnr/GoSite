# API Inventory — Laravel → GoSite REST

> **Canonical contract:** [`api/openapi.yaml`](../api/openapi.yaml) and [`api/examples/`](../api/examples/).
>
> **Plugin permission strings:** [plugin-permissions.md](./plugin-permissions.md).
>
> This document maps **legacy BangunSite routes** to GoSite REST **and** lists **greenfield** endpoints (plugins, terminal, observability) as of **v1.3.1**.

Legacy route mapping to the proposed Go REST API. JSON responses; consistent errors:

```json
{ "error": { "code": "VALIDATION_FAILED", "message": "human-readable message" } }
```

## Proposed conventions

- Panel base URL: `https://host:8080/api/v1`
- Auth: `POST /auth/login` → token/session cookie
- All legacy `/admin/*` endpoints → require auth
- Legacy `/api/server/*` endpoints were **unauthenticated** — in GoSite they **must** be protected

---

## Auth

| Legacy | Method | GoSite |
|--------|--------|--------|
| `GET /` | GET | `GET /auth/login` (metadata: lockscreen enabled, basic auth) |
| `POST /` | POST | `POST /auth/login` `{ email, password, remember }` |
| `GET /locked` | GET | `GET /auth/lockscreen` |
| — | POST | `POST /auth/logout` |
| — | GET | `GET /auth/me` — current user profile |
| — | POST | `POST /auth/lock`, `POST /auth/unlock` |
| BasicAuth middleware | — | `401` + `WWW-Authenticate` when `AUTH_ENABLE=true` |

**Successful login response:**
```json
{ "token": "...", "user": { "id": 1, "name": "Admin", "email": "admin@demo.com" } }
```

---

## Dashboard & monitoring

| Legacy | Method | GoSite |
|--------|--------|--------|
| `GET /admin/` | GET | `GET /dashboard` — initial snapshot |
| `POST /api/server/info` | POST | `GET /system/info` |
| `POST /api/server/traffic` | POST | `GET /system/network` |
| `POST /api/server/diskIO` | POST | `GET /system/disk-io` |
| `POST /api/server/nginx/traffic` | POST | `GET /system/nginx-traffic` |

---

## Website

| Legacy | Method | GoSite |
|--------|--------|--------|
| `GET /admin/website` | GET | `GET /websites` |
| `POST /admin/website` | POST | `POST /websites` |
| `GET /admin/website/{id}/edit` | GET | `GET /websites/{id}` |
| `PUT/PATCH /admin/website/{id}` | PUT | `PUT /websites/{id}` |
| `OPTIONS /admin/website/{id}/enableSite` | PATCH | `PATCH /websites/{id}/toggle` |
| `POST /admin/website/{id}/updateConfig` | POST | `PUT /websites/{id}/nginx-config` |

**Create/update validation** — `POST /websites/validate`

```json
{ "domain", "path", "type", "upstream?", "active", "id?" }
→ { "valid": true } | { "valid": false, "reason": "..." }
```

- Validates domain, path, upstream (proxy), path duplication
- When `active: true`, runs isolated `nginx -t` on the **rendered** config (temp file, **does not** write `site.d`)

| Legacy | Method | GoSite |
|--------|--------|--------|
| `GET /admin/website/{id}/installSSL` | GET | `POST /websites/{id}/ssl/certbot` → `202 { job_id }` |
| — | GET | `GET /websites/{id}/ssl/certbot/stream?job_id=` (SSE) |
| `POST /admin/website/{id}/updateSSL` | POST | `PUT /websites/{id}/ssl/manual` |
| `DELETE /admin/website/{id}/enableSite` | DELETE | `DELETE /websites/{id}?clean=true` |
| `PATCH /admin/website/updateNginx` | PATCH | `PUT /nginx/global` |
| — | GET | `GET /nginx/default` — read default.conf |
| — | PUT | `PUT /nginx/default` |

**Create/update validation** (separate endpoint or inline):
`POST /websites/validate` — see details above.

---

## CLI (boot & ops)

| Command | Notes |
|---------|-------|
| `gosite init` | First-boot storage + migrate + seed |
| `gosite migrate` | Apply migrations |
| `gosite serve` | HTTP API + SPA |
| `gosite nginx-repair` | `nginx -t` + safe auto-fix ([nginx-repair.md](../operations/nginx-repair.md)) |
| `gosite plugin list\|resolve\|install\|catalog` | Plugin operator CLI (wave G) |

Invoked from `config/start.sh` before nginx + `gosite serve`.

---

## Docker

| Legacy | Method | GoSite |
|--------|--------|--------|
| `GET /admin/docker` | GET | `GET /docker/containers` |
| `GET /admin/docker/restart/{id}` | GET | `POST /docker/containers/{id}/restart` |
| `GET /admin/docker/stop/{id}` | GET | `POST /docker/containers/{id}/stop` |
| `GET /admin/docker/log/{id}` | GET | `GET /docker/containers/{id}/logs?tail=200` |

---

## File manager

| Legacy | Method | GoSite |
|--------|--------|--------|
| `GET /admin/browse?path=` | GET | `GET /files?path=/www` |
| `POST /admin/browse/show` | POST | `GET /files/content?path=...` |
| — | GET | `GET /files/raw` — download |
| — | PUT | `PUT /files/content` — save |
| — | POST | `POST /files/batch-save`, `POST /files/batch-delete` |
| `POST /admin/browse/new` | POST | `POST /files` — type: directory\|file\|remote\|upload |
| `PATCH /admin/browse/action` | PATCH | `POST /files/actions` — chmod\|copy\|execute |
| `DELETE /admin/browse/action` | DELETE | `DELETE /files?path=...` |

---

## Mount manager

| Legacy | Method | GoSite |
|--------|--------|--------|
| `GET /admin/mount` | GET | `GET /mounts` |
| `POST /admin/mount` | POST | `POST /mounts` |
| `POST /admin/mount/update` | POST | `PUT /mounts` (identify by device+dir) |
| `GET /admin/mount/enable` | GET | `POST /mounts/enable` |
| `GET /admin/mount/delete` | GET | `DELETE /mounts` |

---

## Cron jobs

| Legacy | Method | GoSite |
|--------|--------|--------|
| `GET /admin/cronjob` | GET | `GET /cronjobs` |
| `POST /admin/cronjob` | POST | `POST /cronjobs` |
| `PUT /admin/cronjob/{id}` | PUT | `PUT /cronjobs/{id}` |
| `DELETE /admin/cronjob/{id}` | DELETE | `DELETE /cronjobs/{id}` |
| `POST /admin/cronjob/run/{id}` | POST | `POST /cronjobs/{id}/run` + stream output |

---

## Settings

| Legacy | Method | GoSite |
|--------|--------|--------|
| `GET /admin/setting` | GET | **Removed** — profile via `GET /auth/me` |
| `POST /admin/setting/update/profile` | POST | `PUT /settings/profile` |
| `POST /admin/setting/update/php` | POST | **Not ported** |
| `POST /admin/setting/update/fpm` | POST | **Not ported** |
| `POST /admin/setting/update/pool` | POST | **Not ported** |

Plugin remote-install host flags: `GET /plugins/install/settings` (read-only env snapshot).

---

## UI metadata

| GoSite | Method | Notes |
|--------|--------|-------|
| `GET /ui/meta` | GET | App name, env label, navigation, auth flags for Preact shell |

## Logs

| Legacy | Method | GoSite |
|--------|--------|--------|
| `GET /admin/logs` | GET | `GET /logs/sites` |
| `GET /admin/logs/get` | GET | `GET /logs?domain=&type=access\|error&tail=1000` |

---

## Database viewer

| Legacy | Method | GoSite |
|--------|--------|--------|
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
|--------|------|--------------|
| GET | `/api/v1/query/meta` | Sources, fields metadata |
| POST | `/api/v1/query` | `{ "source": "audit\|job\|access\|error\|all", "q": "...", "from", "to", "limit" }` |
| GET | `/api/v1/query` | Same params as query string |
| GET | `/api/v1/query/tail` | Live tail |
| GET | `/api/v1/query/saved` | — |
| POST | `/api/v1/query/saved` | `{ "name", "source", "query" }` |
| PATCH | `/api/v1/query/saved/{id}` | Update saved query |
| DELETE | `/api/v1/query/saved/{id}` | Delete saved query |

---

## Grafana Lite (seq 18)

Log-based traffic aggregation. Complements real-time nginx metrics in [seq 22](./sequences/22-nginx-metrics.md).

| Method | Path | Query |
|--------|------|-------|
| GET | `/api/v1/metrics/traffic/series` | `range=24h&step=5m&site=` |
| GET | `/api/v1/metrics/traffic/top-sites` | `range=1h&limit=10` |
| GET | `/api/v1/metrics/traffic/status-codes` | `range=24h&site=` |
| GET | `/api/v1/metrics/traffic/summary` | `range=1h` |

---

## Nginx metrics (seq 22 — stub_status + VTS)

Co-located collectors; no Prometheus. Session required; scope `metrics:read` for plugins.

| Method | Path | Query | Notes |
|--------|------|-------|-------|
| GET | `/api/v1/metrics/nginx/current` | — | Latest stub_status + request rate |
| GET | `/api/v1/metrics/nginx/series` | `range=1h` | Connection + rate series |
| GET | `/api/v1/metrics/nginx/vts/status` | — | VTS enabled probe |
| GET | `/api/v1/metrics/nginx/vts/servers` | `limit=10` | Per-`server_name` (latest snapshot) |
| GET | `/api/v1/metrics/nginx/vts/upstreams` | `limit=10` | Per-upstream peer (latest snapshot) |

Env: `GOSITE_NGINX_STUB_STATUS_URL`, `GOSITE_NGINX_VTS_URL`. OpenAPI: `api/openapi.yaml` (Metrics tag). See [22-nginx-metrics.md](../sequences/22-nginx-metrics.md).

---

## Nginx ops

| Method | Path | Notes |
|--------|------|-------|
| POST | `/api/v1/nginx/reload` | `TestAndRepair` then `nginx -s reload` |
| POST | `/api/v1/nginx/test` | Test raw config body |
| GET/PUT | `/api/v1/nginx/global` | `nginx.conf` |
| GET/PUT | `/api/v1/nginx/default` | `http.d/default.conf` |

Every internal reload calls [nginx auto-repair](../operations/nginx-repair.md) first.

---

## Error format (implemented)

```json
{ "error": { "code": "NGINX_TEST_FAILED", "message": "..." } }
```

---

## Endpoints that need streaming / long-poll

Legacy used `/tmp/*.txt` file polling for async output:

| Feature | GoSite |
|---------|--------|
| Certbot install | SSE `GET /websites/{id}/ssl/certbot/stream?job_id=` — `done` event when finished |
| Cron manual run | SSE `GET /cronjobs/{id}/run/stream` |
| Docker logs | Optional SSE for follow mode |

---

## Migration security notes

1. Change all action `GET` routes (docker restart/stop) → `POST`
2. Protect `/system/*` with auth (legacy was open)
3. File manager & mount: strict path validation, deny outside allowlisted roots
4. Cron payload: consider allowlist or approval for dangerous commands

---

## Floating Terminal (xterm.js, topbar popup)

| GoSite | Method | Notes |
|--------|--------|-------|
| `GET /terminal/ws?session_id=&cols=&rows=` | WS upgrade | PTY session; first attach is writer, others reader |
| `GET /terminal/sessions` | GET | List sessions owned by the authenticated user |
| `GET /terminal/sessions/{id}/snapshot` | GET | Rolling dump + first/end seq (base64 data) |
| `DELETE /terminal/sessions/{id}` | DELETE | Terminate the session and remove its dump file |

Wire format: text frames for control (`ready` / `exit` / `error` / `pong` /
`input` / `resize` / `ping` / `replay`); binary frames prefixed with an 8-byte
big-endian monotonic sequence number carry raw PTY output.

See [`docs/sequences/10-floating-terminal.md`](sequences/10-floating-terminal.md)
for the full lifecycle (initial attach, refresh, server restart, sweeper
kill, multi-attach).

---

## Plugins

Greenfield REST (no Laravel equivalent). Canonical detail: `api/openapi.yaml` (base install routes) + [sequence 19](sequences/19-plugin-installer.md) / [sequence 20](sequences/20-plugin-remote-distribution-impl.md). Permission registry: [plugin-permissions.md](reference/plugin-permissions.md).

| GoSite | Method | Purpose |
|--------|--------|---------|
| `GET /plugins` | GET | Installed registry |
| `POST /plugins/install` | POST | Multipart zip **or** JSON `{ "source": … }` **or** manifest |
| `POST /plugins/install/resolve` | POST | Preview remote source (URL, GitHub, GitLab, catalog ref) |
| `GET /plugins/install/settings` | GET | Remote install flags (read-only from env) |
| `GET /plugins/catalog` | GET | Bundled curated catalog search |
| `GET /plugins/catalog/{vendor}/{name}` | GET | One catalog entry |
| `POST /plugins/{vendor}/{name}/enable` | POST | Enable version |
| `POST /plugins/{vendor}/{name}/disable` | POST | Disable |
| `POST /plugins/{vendor}/{name}/switch` | POST | Switch active version |
| `DELETE /plugins/{vendor}/{name}/versions/{version}` | DELETE | Uninstall / purge |
| `GET/PUT /plugins/{vendor}/{name}/versions/{version}/config` | GET/PUT | Encrypted plugin config |
| `GET/POST/DELETE /plugins/keyring` | * | Trusted signing keys (admin) |

CLI (same APIs): `gosite plugin list|resolve|install|catalog`. Detail: `api/openapi.yaml`.
