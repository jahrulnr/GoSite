# Sequence 23 — Implementation plan (wave B)

Companion to [23-builtin-plugins.md](./23-builtin-plugins.md). **Status:** B1–B2 implemented (B3 restore API partial)

> Wave index: [WAVE-PLUGIN-B.md](../implementation/WAVE-PLUGIN-B.md)

## Wave B1 — Infrastructure

| ID | Deliverable | Done when | Status |
|----|-------------|-----------|--------|
| B1-1 | `internal/service/plugin/bundled/` + embedded index | Load index + read artifact bytes | ⬜ |
| B1-2 | Migration 009 — document `bundled` provenance | `source_type=bundled` stored on seed | ⬜ |
| B1-3 | `SeedBundled(ctx)` on `plugin.Service` | Idempotent; reuses `Install` | ⬜ |
| B1-4 | Trust bypass for `bundled` in `verifyArtifact` | Tests: seed without external signature | ⬜ |
| B1-5 | Hook `bootstrap.Init` + `Reconcile` prefix | Fresh DB has bundled rows | ⬜ |
| B1-6 | Config: `PLUGIN_BUNDLED_*` env vars | `config.go` + docs | ⬜ |
| B1-7 | Dockerfile build stage | Image contains `/app/bundled-plugins/` | ⬜ |

### B1-1 — Bundled package

- [ ] `internal/service/plugin/bundled/index.json` (embedded default)
- [ ] `bundled.Service` — `List()`, `LoadArtifact(pluginID)`, path override
- [ ] Unit tests: missing artifact, bad JSON

### B1-2 — Provenance

- [ ] Migration `009_plugin_bundled.sql` (if column constraint needed; else doc-only)
- [ ] `InstallInput.Provenance` accepts `SourceType: "bundled"`
- [ ] Repository read/write unchanged (007 columns sufficient)

### B1-3 — SeedBundled

- [ ] `func (s *Service) SeedBundled(ctx context.Context) error`
- [ ] Skip when `PLUGIN_BUNDLED_ENABLED=false`
- [ ] Digest compare before re-install
- [ ] `permissions_pre_ack` → `PermissionsAck: true` on install
- [ ] Tests: empty DB seeds one row; second call no-op; digest change adds version

### B1-4 — Trust

- [ ] `verifyArtifact`: if provenance `bundled`, skip signature or use host key
- [ ] Test strict mode + bundled seed succeeds

### B1-5 — Hooks

- [ ] `bootstrap.Init` calls `SeedBundled` after migrate
- [ ] `Reconcile` calls `SeedBundled` first (or shared startup in router)
- [ ] Optional `PLUGIN_BUNDLED_AUTO_ENABLE` → enable after seed (non-prod only)

### B1-6 — Config

- [ ] `PluginBundledEnabled`, `PluginBundledPath`, `PluginBundledAutoEnable`
- [ ] Reference in `docs/reference/` or plugin env table

### B1-7 — Docker / Makefile

- [ ] `make -C plugins/gosite/mcp build` in Dockerfile gobuilder
- [ ] `COPY` to `/app/bundled-plugins`
- [ ] Root `make bundled-plugins` for local dev

## Wave B2 — `gosite/mcp` first built-in

| ID | Deliverable | Done when | Status |
|----|-------------|-----------|--------|
| B2-1 | Index entry `gosite/mcp` + zip in image | Artifact matches Makefile output | ⬜ |
| B2-2 | Manual smoke: init → list plugins | `gosite/mcp` `installed`, disabled | ⬜ |
| B2-3 | UI: Built-in badge + empty state copy | `Plugins.tsx` | ⬜ |
| B2-4 | Detail aside: bundled provenance | Source card | ⬜ |
| B2-5 | Update `mcp-operator.md` step 0 | "Already installed — enable" | ⬜ |
| B2-6 | Integration route smoke | Enable → `/plugins/gosite/mcp/integration` | ⬜ |

### B2-3 — Panel

- [ ] Badge when `source_type === 'bundled'`
- [ ] Empty state CTA for built-in MCP
- [ ] Types in `web/src/api/types.ts` if needed

## Wave B3 — Upgrade & restore

| ID | Deliverable | Done when | Status |
|----|-------------|-----------|--------|
| B3-1 | Seed on upgrade detects digest bump | New version row | ⬜ |
| B3-2 | UI: "Built-in update available" | When newer bundled digest exists | ⬜ |
| B3-3 | `POST /plugins/{id}/restore-bundled` | OpenAPI + handler | ⬜ |
| B3-4 | Audit `plugin.bundled_seeded`, `plugin.bundled_restored` | Splunk-lite | ⬜ |

### B3-3 — Restore API

- [ ] Handler on `plugin.go`
- [ ] `restorable` check from bundled index
- [ ] UI button on uninstalled / missing built-in rows

## Wave B4 — Deferred

- [ ] Second official plugin (e.g. tier-0 observability webhook)
- [ ] Catalog `bundled: true` on `gosite/mcp` entry
- [ ] `go:embed` multi-platform zips for non-Docker binaries

## Gate (wave B complete)

Wave B2 minimum ship when:

1. `go test -race -count=1 ./internal/service/plugin/...` exits 0
2. Docker image seed test: empty volume → `GET /api/v1/plugins` includes `gosite/mcp` `installed`
3. Enable + integration token smoke passes
4. `mcp-operator.md` updated

## Backend implementation map

| Component | Package / file | Responsibility |
|-----------|----------------|----------------|
| Bundled index | `internal/service/plugin/bundled/` | Embed + load artifacts |
| Seed | `internal/service/plugin/bundled_seed.go` | `SeedBundled` |
| Install reuse | `service.go` | `Install` + bundled provenance |
| Bootstrap | `internal/bootstrap/init.go` | Call seed after migrate |
| Router | `internal/delivery/http/router.go` | Reconcile order |
| Config | `internal/config/config.go` | `PLUGIN_BUNDLED_*` |
| Restore handler | `handler/plugin.go` | `restore-bundled` (B3) |
| Dockerfile | `Dockerfile` | Build + copy bundled zips |
| Panel | `web/src/views/Plugins.tsx` | Badge, empty state |

## References

- [23-builtin-plugins.md](./23-builtin-plugins.md)
- [WAVE-PLUGIN-B.md](../implementation/WAVE-PLUGIN-B.md)
- [21-plugin-mcp-impl.md](./21-plugin-mcp-impl.md)
