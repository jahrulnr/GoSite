# Plugin permissions & integration scopes

**Status:** Canonical registry (v1.4 target). **Runtime enforcement** for integration tokens (`gs_pat_*`) is planned in [sequence 21](../sequences/21-plugin-mcp.md) (P6-host-auth). Today manifest `permissions` are **declared and acked** at install; the host does not yet reject API calls by scope string.

**Source of truth for routes:** `internal/delivery/http/router.go` · **Contract:** [`api/openapi.yaml`](../../api/openapi.yaml)

## How permissions work

```text
Plugin manifest permissions[]     ← ceiling (install ack, permissions_acked_caps)
        ↓ subset
Integration token scopes[]        ← operator whitelist (generate / edit)
        ↓
API middleware + MCP tools/list   ← enforcement (P6-host-auth, not shipped yet)
```

| Layer | Who sets it | Stored in |
|-------|-------------|-----------|
| Manifest | Plugin author | `manifest.json` → `plugin_versions.manifest_json` |
| Install ack | Operator | `permissions_ack_at`, `permissions_acked_caps` |
| Token scopes | Operator (session) | `plugin_access_tokens` (planned) |

Human panel users still use **session auth** (full access to all routes their login allows). Integration tokens are **scoped**; they never impersonate a full panel user.

## Naming convention

```text
<domain>:<level>
```

| Level | Meaning | Examples |
|-------|---------|----------|
| `read` | List, get, dry-run, stream read | `websites:read`, `jobs:read` |
| `write` | Create, update, delete non-critical resources | `websites:write`, `cron:write` |
| `manage` | Host-impact actions (reload, restart, run job, terminal) | `nginx:manage`, `docker:manage` |

**Rules**

- Use only strings from the registry below (or documented plugin-runtime extensions).
- Unknown permission strings should be rejected at install preview (planned validator).
- Token `scopes[]` must be ⊆ manifest `permissions[]` for that plugin version.
- Prefer **narrow** manifests; operators can still grant full manifest via token presets.

## Risk tiers (UI grouping)

| Tier | Label | Scope levels | Operator default |
|------|-------|--------------|------------------|
| T0 | Read | `*:read` | Pre-checked when generating MCP token |
| T1 | Write | `*:write` | Opt-in |
| T2 | Manage | `*:manage` | Opt-in + confirm |
| T3 | Host-critical | `nginx:manage`, `terminal:manage`, `plugins:manage`, `files:manage` | Strong confirm |

## Canonical scope registry

All routes require session (or future access token) on `gosite serve :8080` — see [architecture overview](../architecture/overview.md). `GET /health` is public (no scope).

### Auth & profile

| Scope | Routes | Notes |
|-------|--------|-------|
| — | `GET/POST /auth/login`, `GET /auth/login` | Public (behind optional Basic Auth) |
| — | `POST /auth/logout`, `GET /auth/me`, lockscreen | Session only — **not** grantable to integration tokens |
| `settings:write` | `PUT /settings/profile` | User profile update |

### Dashboard & system

| Scope | Routes | MCP (planned) |
|-------|--------|---------------|
| `dashboard:read` | `GET /dashboard` | — |
| `system:read` | `GET /system/info`, `/system/network`, `/system/disk-io`, `/system/nginx-traffic` | `system` tool |
| `ui:read` | `GET /ui/meta` | — |

### Websites

| Scope | Routes | MCP (planned) |
|-------|--------|---------------|
| `websites:read` | `GET /websites`, `GET /websites/{id}`, `POST /websites/validate`, `GET /websites/{id}/nginx-config` | `websites` read actions |
| `websites:write` | `POST /websites`, `PUT /websites/{id}`, `DELETE /websites/{id}`, `PATCH /websites/{id}/toggle`, `PUT /websites/{id}/nginx-config`, `POST /websites/{id}/nginx-config/test` | `websites` mutate actions |

### SSL

| Scope | Routes | MCP (planned) |
|-------|--------|---------------|
| `ssl:read` | `GET /websites/{id}/ssl`, `GET /websites/{id}/ssl/certbot/stream` | — |
| `ssl:write` | `PUT /websites/{id}/ssl/manual`, `POST /websites/{id}/ssl/certbot` | — |

### Nginx (global & reload)

| Scope | Routes | MCP (planned) |
|-------|--------|---------------|
| `nginx:read` | `GET /nginx/default`, `GET /nginx/global`, `POST /nginx/test` | `nginx` test-only mode |
| `nginx:manage` | `PUT /nginx/default`, `PUT /nginx/global`, `POST /nginx/reload` | `nginx` reload |

Gosite **manages** website nginx (`site.d` / `active.d`) via website routes; global nginx ops use scopes above. Website edge (`nginx :80/:443`) is not the panel auth path.

### Docker

| Scope | Routes | MCP (planned) |
|-------|--------|---------------|
| `docker:read` | `GET /docker/containers`, `GET /docker/containers/{id}/logs` | `docker` list/logs |
| `docker:manage` | `POST /docker/containers/{id}/restart`, `POST /docker/containers/{id}/stop` | `docker` restart/stop |

### Files

| Scope | Routes | Notes |
|-------|--------|-------|
| `files:read` | `GET /files`, `GET /files/content`, `GET /files/raw` | Root allowlist in service |
| `files:write` | `PUT /files/content`, `POST /files`, `POST /files/batch-save` | |
| `files:manage` | `POST /files/actions` (chmod, copy, execute), `POST /files/batch-delete`, `DELETE /files` | Execute gated by `FILES_ALLOW_EXECUTE` |

### Mounts

| Scope | Routes |
|-------|--------|
| `mounts:read` | `GET /mounts` |
| `mounts:write` | `POST /mounts`, `PUT /mounts`, `DELETE /mounts`, `POST /mounts/enable` |

### Cron & jobs

| Scope | Routes | MCP (planned) |
|-------|--------|---------------|
| `cron:read` | `GET /cronjobs` | — |
| `cron:write` | `POST /cronjobs`, `PUT /cronjobs/{id}`, `DELETE /cronjobs/{id}` | — |
| `cron:manage` | `POST /cronjobs/{id}/run`, `GET /cronjobs/{id}/run/stream` | — |
| `jobs:read` | *(via query source `job`)* `POST/GET /query` with `source=job` | `jobs` tool |

Job worker events are also visible in Splunk-lite query UI; scope `query:read` covers search.

### Logs (file tail)

| Scope | Routes | Notes |
|-------|--------|-------|
| `logs:read` | `GET /logs/sites`, `GET /logs?domain=&type=` | Per-site access/error logs |

### Observability (Splunk-lite & Grafana-lite)

| Scope | Routes |
|-------|--------|
| `query:read` | `GET /query/meta`, `GET/POST /query`, `GET /query/tail`, `GET /query/saved` |
| `query:write` | `POST /query/saved`, `PATCH /query/saved/{id}`, `DELETE /query/saved/{id}` |
| `metrics:read` | `GET /metrics/traffic/series`, `top-sites`, `status-codes`, `summary` |

### Database viewer

| Scope | Routes |
|-------|--------|
| `database:read` | `GET /database/tables`, `GET /database/tables/{name}` |

### Terminal

| Scope | Routes | Notes |
|-------|--------|-------|
| `terminal:manage` | `GET /terminal/ws`, `GET /terminal/sessions`, `GET /terminal/sessions/{id}/snapshot`, `DELETE /terminal/sessions/{id}` | Host-critical — PTY on container |

### Plugins

| Scope | Routes | MCP (planned) |
|-------|--------|---------------|
| `plugins:read` | `GET /plugins`, `GET /plugins/catalog`, `GET /plugins/catalog/{vendor}/{name}`, `GET /plugins/install/settings`, `GET /plugins/{vendor}/{name}/versions/{version}/config` | `plugins` list meta |
| `plugins:manage` | `POST /plugins/install/resolve`, `POST /plugins/install`, enable/disable/switch/uninstall, `PUT` config, `GET/POST/DELETE /plugins/keyring` | — |

Token CRUD for integration (`POST …/integration-tokens`) is **session-only** — not delegable to `gs_pat_*`.

## Plugin-runtime permissions (manifest only)

These do not map 1:1 to REST routes today; they declare **runtime capability** for tier 0/1 plugins.

| Permission | Meaning | Enforcement today |
|------------|---------|-------------------|
| `network.outbound` | Plugin may call external HTTP(S) | Documented operator responsibility; egress policy deferred (seq 19) |
| `secrets:receive` | Host may pass decrypted config secrets into hook/runtime | Config service + tier-1 RPC |
| `hooks:*` | *(implicit)* | Declared via `capabilities.hooks[]`, not `permissions[]` |

## Deprecated / legacy strings

Do not use in new manifests. Host validator should map or reject.

| Legacy (docs/examples) | Replace with |
|------------------------|--------------|
| `nginx:reload:read-only` | `nginx:read` (test/get) — reload requires `nginx:manage` |

Update seq 19 examples to the canonical names when touching that file.

## Reference manifest: `gosite/mcp` (planned)

MCP catalog plugin — subset of full registry ([sequence 21](../sequences/21-plugin-mcp.md)):

```json
"permissions": [
  "system:read",
  "websites:read",
  "websites:write",
  "nginx:read",
  "nginx:manage",
  "docker:read",
  "docker:manage",
  "jobs:read",
  "plugins:read"
]
```

Split `nginx:read` vs `nginx:manage` so operators can issue test-only tokens without reload.

### MCP tool ↔ scope mapping

| MCP tool | Required scope(s) |
|----------|---------------------|
| `system` | `system:read` |
| `websites` | `websites:read`; mutations need `websites:write` |
| `nginx` | `nginx:read` for test; `nginx:manage` for reload |
| `docker` | `docker:read`; restart/stop need `docker:manage` |
| `jobs` | `jobs:read` (alias: `query:read` + job source — implement as `jobs:read`) |
| `plugins` | `plugins:read` |

## UI presets (token generator)

| Preset | Scopes |
|--------|--------|
| **Read only** | All `*:read` in manifest + `jobs:read` |
| **Operator** | Read + `websites:write`, `docker:manage`, `nginx:read` |
| **Full manifest** | All permissions declared in manifest |

## Implementation checklist (P6-host-auth)

- [ ] `pkg/pluginperm` — registry constant + `Valid(scope string) bool`
- [ ] Install/resolve — warn on unknown `permissions[]`
- [ ] Middleware — `RequireScope("websites:read")` per route group
- [ ] `GET /api/v1/plugins/permissions/registry` — machine-readable list for UI picker
- [ ] OpenAPI — integration token paths + scope enum
- [ ] Tests — token without scope gets 403 on protected route

## Related docs

| Topic | Document |
|-------|----------|
| MCP operator flow | [21-plugin-mcp.md](../sequences/21-plugin-mcp.md) |
| Plugin platform | [plugin-platform.md](../architecture/plugin-platform.md) |
| Installer & ack | [19-plugin-installer.md](../sequences/19-plugin-installer.md) |
| API routes | [api-inventory.md](./api-inventory.md) |
