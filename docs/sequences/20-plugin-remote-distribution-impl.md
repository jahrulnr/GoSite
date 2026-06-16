# Sequence 20 — Implementation plan (wave G)

Companion to [20-plugin-remote-distribution.md](./20-plugin-remote-distribution.md). **Status:** In progress.

## First ship scope

| Phase | In first ship? | Notes |
|-------|--------------|-------|
| **G1** URL + fetcher + provenance migration | Yes | Foundation |
| **G2** GitHub + `gosite.plugin.json` | Yes | Primary remote path |
| **G1c** Permission install prompt | Yes | Panel + API ack |
| **G1b** Keyring UI | Optional | API exists; UI can follow |
| **G2b** Docker builder | **No** | Highest risk — after G1+G2 stable |
| **G3–G6** | Later | GitLab, catalog, CLI |

## PR slices (recommended order)

### PR-1 — Foundation (this branch)

- [x] `migrations/007_plugin_provenance.sql`
- [x] `internal/service/plugin/remote/` — types, URL fetcher, op lock
- [x] `internal/config` — `PLUGIN_FETCH_*`, `PLUGIN_REMOTE_INSTALL`, allowlist
- [x] `POST /api/v1/plugins/install/resolve` (URL source)
- [x] `POST /api/v1/plugins/install` with `{ "source": { "type": "url", ... } }`
- [ ] SQLite repo: read/write provenance columns
- [x] Tests: fetcher allowlist, redirect, digest mismatch

### PR-2 — GitHub resolver (G2)

- [ ] `remote/resolver/github.go` — releases API + fetch `gosite.plugin.json` at tag
- [ ] Asset selection by `GOOS/ARCH` + pinned `sha256`
- [ ] Panel: GitHub tab + resolve preview
- [ ] `GITHUB_TOKEN` from config (private repos)
- [ ] Failure classes: `release_integrity_failed`, `platform_unsupported`, `auth_token_expired`

### PR-3 — Permission prompt (G1c)

- [ ] API: `permissions_ack` on install request
- [ ] `permissions_acked_caps` snapshot + re-ack on capability diff
- [ ] Enable gate until ack recorded
- [ ] Panel: capability list in preview card

### PR-4 — Lifecycle hardening

- [ ] `OpLock` wired into `Service` install/enable/disable/switch/uninstall
- [ ] `409 operation_in_progress`
- [ ] Install operation log (`install_log` JSON column)
- [ ] TOCTOU: `resolveToken` TTL + fail install on digest drift

### PR-5 — UI polish

- [ ] Install hub wizard (URL + GitHub tabs)
- [ ] Provenance column in registry table
- [ ] Settings → Plugins (token status read-only)

### PR-6 — G2b (deferred)

- [ ] Docker builder image `gosite/builder:go`
- [ ] `distribution.build` contract
- [ ] Build quotas + cleanup policy

## Package layout

```text
internal/service/plugin/
  service.go              # existing Install(bytes) — unchanged core
  oplock.go               # per-plugin_id mutex (PR-4 wire)
  remote/
    types.go              # Source, FetchPlan, ResolvePreview
    service.go            # RemoteService: Resolve, FetchAndInstall
    failures.go           # failure_class constants
    fetch/
      fetcher.go          # HTTP GET, allowlist, redirects, size cap
      fetcher_test.go
    resolver/
      resolver.go         # Registry + interface
      url.go              # G1
      github.go           # G2 (PR-2)
```

## API contract (v1)

### Resolve

```http
POST /api/v1/plugins/install/resolve
{ "source": { "type": "url", "url": "https://…", "sha256": "abc…" } }
```

Response: `ResolvePreview` — manifest hints, digest, size (HEAD or index), permissions, no full zip body.

### Install (extended)

```http
POST /api/v1/plugins/install
{
  "source": { "type": "url", "url": "https://…", "sha256": "abc…" },
  "permissions_ack": true
}
```

Multipart upload **unchanged**.

## Config (env)

| Variable | Default | Phase |
|----------|---------|-------|
| `PLUGIN_REMOTE_INSTALL` | `true` | G1 |
| `PLUGIN_INSTALL_ALLOWED_HOSTS` | `github.com,…` | G1 |
| `PLUGIN_FETCH_MAX_BYTES` | `67108864` | G1 |
| `PLUGIN_FETCH_TIMEOUT` | `120s` | G1 |
| `PLUGIN_FETCH_MAX_REDIRECTS` | `3` | G1 |
| `PLUGIN_TRUST_MODE` | `strict` (prod) | G1 |
| `GITHUB_TOKEN` | — | G2 |
| `PLUGIN_BUILD_*` | — | G2b only |

## Existing code touchpoints

| File | Change |
|------|--------|
| `internal/delivery/http/handler/plugin.go` | Resolve, source branch on Install |
| `internal/delivery/http/router.go` | Route `POST /plugins/install/resolve` |
| `internal/repository/sqlite/plugin.go` | Provenance columns (PR-1 finish) |
| `web/src/views/Plugins.tsx` | Install hub (PR-5) |
| `web/src/api/endpoints.ts` | `resolvePluginInstall`, `installFromSource` |

## Validation commands

```bash
go test ./internal/service/plugin/...
go test ./internal/delivery/http/handler/...
cd web && npm test
```

## References

- [20-plugin-remote-distribution.md](./20-plugin-remote-distribution.md) — design
- [19-plugin-installer.md](./19-plugin-installer.md) — lifecycle, signatures
