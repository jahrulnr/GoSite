# Permission & scope plugin

**Status:** Registry kanonik (target v1.4). **Enforcement** token integrasi (`gs_pat_*`) direncanakan di [sequence 21](../sequences/21-plugin-mcp.md) (P6-host-auth) — detail [integration-tokens.md](./integration-tokens.md). Saat ini `permissions` manifest hanya **dideklarasikan dan di-ack** saat install.

**Sumber route:** `internal/delivery/http/router.go` · EN lengkap: [plugin-permissions.md](./plugin-permissions.md)

## Ringkas

```text
manifest permissions[]  →  plafon (ack install)
        ↓ subset
token scopes[]          →  whitelist operator (generate / edit)
        ↓
API + MCP tools/list    →  enforcement (belum diimplementasi)
```

## Konvensi penamaan

```text
<domain>:<level>     level = read | write | manage
```

| Tier UI | Contoh |
|---------|--------|
| T0 Read | `system:read`, `websites:read`, `jobs:read` |
| T1 Write | `websites:write`, `cron:write` |
| T2 Manage | `docker:manage`, `nginx:manage` |
| T3 Kritis | `terminal:manage`, `plugins:manage`, `files:manage` |

## Registry scope (ringkas)

| Domain | read | write | manage |
|--------|------|-------|--------|
| `system` | info, network, disk-io, nginx-traffic | — | — |
| `websites` | list, get, validate, get nginx-config | create, update, delete, toggle, nginx-config | — |
| `ssl` | status, certbot stream | manual, certbot start | — |
| `nginx` | default, global, test | — | update default/global, **reload** |
| `docker` | containers, logs | — | restart, stop |
| `files` | browse, content, raw | save, create, batch-save | actions, delete, execute |
| `mounts` | list | create, update, delete, enable | — |
| `cron` | list | create, update, delete | run + stream |
| `jobs` | query source `job` | — | — |
| `logs` | sites, tail | — | — |
| `query` | meta, search, tail, saved | saved CRUD | — |
| `metrics` | traffic series/summary | — | — |
| `database` | tables read-only | — | — |
| `terminal` | — | — | WS, sessions, kill |
| `plugins` | list, catalog, config get | — | install, enable, keyring, … |

Panel/MCP memanggil **`gosite :8080`** langsung — nginx `:80`/`:443` hanya website edge ([overview](../architecture/overview.md)).

## Runtime plugin (bukan REST)

| Permission | Arti |
|------------|------|
| `network.outbound` | HTTP keluar tier 0/1 (egress deferred) |
| `secrets:receive` | Secret config ke runtime/hook |

## Legacy → kanonik

| Lama | Ganti |
|------|-------|
| `nginx:reload:read-only` | `nginx:read` (+ `nginx:manage` untuk reload) |

## Manifest `gosite/mcp` (rencana)

`system:read`, `websites:read/write`, `nginx:read`, `nginx:manage`, `docker:read/manage`, `jobs:read`, `plugins:read` — detail mapping MCP: lihat EN.

## Preset token UI

- **Read only** — semua `*:read` di manifest
- **Full manifest** — semua permission plugin

## Terkait

- [21-plugin-mcp.md](../sequences/21-plugin-mcp.md)
- [plugin-platform.md](../architecture/plugin-platform.md)
- [api-inventory.md](./api-inventory_id.md)
