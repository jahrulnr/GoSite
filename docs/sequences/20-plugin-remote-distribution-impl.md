# Sequence 20 — Implementation plan (wave G)

Companion to [20-plugin-remote-distribution.md](./20-plugin-remote-distribution.md). **Status:** First ship complete (G1+G2+G1c)

## First ship scope

| Phase | Status | Notes |
|-------|--------|-------|
| **G1** URL + fetcher + provenance | Done | |
| **G2** GitHub + `gosite.plugin.json` | Done | `github-release` resolver |
| **G1c** Permission install prompt | Done | API + panel |
| **G1b** Keyring UI | Done | Registry + `/plugins/keyring` tab |
| **G2b** Docker builder | Deferred | Per plan |
| **G3–G6** | Deferred | GitLab, catalog, CLI |

## PR-1 — Foundation

- [x] `migrations/007_plugin_provenance.sql`
- [x] `internal/service/plugin/remote/` — types, URL fetcher, op lock
- [x] `internal/config` — `PLUGIN_FETCH_*`, `PLUGIN_REMOTE_INSTALL`, allowlist
- [x] `POST /api/v1/plugins/install/resolve` (URL + GitHub)
- [x] `POST /api/v1/plugins/install` with `{ "source": ... }`
- [x] SQLite repo: read/write provenance columns
- [x] Tests: fetcher allowlist, digest mismatch, index parser

## PR-2 — GitHub resolver (G2)

- [x] `remote/resolver/github.go` — raw `gosite.plugin.json` + `manifest.json` at tag
- [x] Asset selection by `GOOS/ARCH` + pinned `sha256`
- [x] Panel: GitHub tab + resolve preview (subagent)
- [x] `GITHUB_TOKEN` from config
- [x] Failure classes: `auth_token_expired`, `platform_unsupported`, `resolve_failed`

## PR-3 — Permission prompt (G1c)

- [x] API: `permissions_ack` on remote install request
- [x] `permissions_acked_caps` snapshot on install
- [x] Enable gate for remote installs without ack
- [x] Panel: capability list + ack checkbox

## PR-4 — Lifecycle hardening

- [x] `OpLock` wired into install/enable/disable/switch/uninstall
- [x] `409 operation_in_progress` (`failure_class=operation_in_progress`)
- [x] Install operation log steps (`installlog.go` + `SetInstallLog`)
- [x] TOCTOU: `resolveToken` TTL cache + panel passes token on install

## PR-5 — UI polish

- [x] Install hub wizard (URL + GitHub tabs)
- [x] Provenance column in registry table
- [x] Distribution card in detail panel
- [x] Install log in detail panel
- [x] Keyring admin tab (`PluginsKeyring.tsx`)
- [x] `GET /plugins/install/settings`
- [ ] Settings page token UI (deferred)

## API endpoints

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/plugins/install/settings` | Remote install config snapshot |
| POST | `/plugins/install/resolve` | Lightweight preview |
| POST | `/plugins/install` | Upload, manifest, or `{source}` |

## Validation

```bash
go test ./internal/service/plugin/...
cd web && npm test
```
