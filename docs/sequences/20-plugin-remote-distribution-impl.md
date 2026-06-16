# Sequence 20 — Implementation plan (wave G)

Companion to [20-plugin-remote-distribution.md](./20-plugin-remote-distribution.md). **Status:** Wave G complete

## Scope

| Phase | Status | Notes |
|-------|--------|-------|
| **G1** URL + fetcher + provenance | Done | |
| **G2** GitHub + `gosite.plugin.json` | Done | prefer-release + dual-path build fallback |
| **G1c** Permission install prompt | Done | API + panel |
| **G1b** Keyring UI | Done | Registry + `/plugins/keyring` tab |
| **G2b** Docker builder | Done | `PLUGIN_BUILD_*`, `github-build` / `gitlab-build`, dual path |
| **G3** GitLab resolver | Done | `gitlab-release` + panel tab |
| **G4** Catalog | Done | bundled JSON + API + install catalog tab |
| **G5** git-ref tier-0 | Done | `git-ref` resolver |
| **G6** CLI | Done | `gosite plugin list|resolve|install|catalog` |

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

- [x] Install hub wizard (URL + GitHub + GitLab + Catalog tabs)
- [x] Provenance column in registry table
- [x] Distribution card in detail panel
- [x] Install log in detail panel
- [x] Keyring admin tab (`PluginsKeyring.tsx`)
- [x] `GET /plugins/install/settings`
- [x] Settings page token status (read-only; host env `GITHUB_TOKEN`)

## G2b — Docker builder

- [x] `distribution.build` in index parser
- [x] `remote/build/docker.go` — git clone + `docker run` golang image
- [x] Dual path: release asset preferred, build fallback when `PLUGIN_BUILD_ENABLED`
- [x] Source types `github-build`, `gitlab-build`

## G3 — GitLab

- [x] `remote/resolver/gitlab.go`
- [x] Panel GitLab tab
- [x] `GITLAB_TOKEN` host env

## G4 — Catalog

- [x] `internal/service/plugin/catalog/` — bundled `catalog.json` + override path
- [x] `GET /plugins/catalog`, `GET /plugins/catalog/:vendor/:name`
- [x] Install modal catalog tab

## G5 — git-ref

- [x] `remote/resolver/gitref.go` — tier-0 manifest zip inline artifact

## G6 — CLI

- [x] `gosite plugin list|resolve|install|catalog`

## API endpoints

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/plugins/install/settings` | Remote install config snapshot |
| GET | `/plugins/catalog` | Curated plugin search |
| GET | `/plugins/catalog/:vendor/:name` | One catalog entry |
| POST | `/plugins/install/resolve` | Lightweight preview |
| POST | `/plugins/install` | Upload, manifest, or `{source}` |

## Validation

```bash
go test ./internal/service/plugin/...
cd web && npm test
```
