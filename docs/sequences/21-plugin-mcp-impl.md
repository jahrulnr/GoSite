# Sequence 21 — Implementation plan (wave P6)

Companion to [21-plugin-mcp.md](./21-plugin-mcp.md). **Status:** P6-host-auth + P6b monorepo implemented; P6a external repo pending; P6c blocked

> Wave index: [WAVE-PLUGIN-P6.md](../implementation/WAVE-PLUGIN-P6.md)

## P6-host-auth — gates

| ID | Deliverable | Done when | Status |
|----|-------------|-----------|--------|
| H1 | Migration `plugin_access_tokens` + switch reconciliation hook | CRUD in repository; truncate/revoke on manifest shrink | ✅ |
| H2 | Token API (create, list, PATCH scopes, revoke, self) | OpenAPI + handler tests | ✅ |
| H3 | `RequireSessionOrAccessToken` + per-route scopes | Integration tests on 2+ routes | ✅ |
| H4 | Plugin UI: generate, edit scopes, revoke, mcp.json copy | E2E manual checklist | ✅ |
| H5 | Audit events (incl. `scopes_truncated`; `used` dedup 60s) | Splunk-lite queryable | ✅ |

### H1 — Database & switch hook

- [x] `migrations/008_plugin_access_tokens.sql`
- [x] `plugin_access_tokens` repository CRUD
- [x] Hook after `enableUnlocked` — reconciliation when manifest changes
- [x] Disable → suspend (401); uninstall → hard-revoke when no versions remain
- [x] [domain-model.md](../architecture/domain-model.md)

### H2 — Token API

- [x] `POST/GET/PATCH/DELETE /api/v1/plugins/{id}/integration-tokens`
- [x] `GET /api/v1/integration-tokens/self`
- [x] OpenAPI paths + schemas
- [x] Scope validation ⊆ manifest on create/patch

### H3 — Middleware

- [x] `RequireSessionOrAccessToken` middleware
- [x] Per-route scope enforcement (`system:read`, `websites:*`, `nginx:*`, `docker:*`)
- [x] `last_used_at` on every successful access-token request (middleware `RecordUse`)

### H4 — Plugin UI

- [x] Integration tab at `/plugins/gosite/mcp/integration`
- [x] Generate + scope picker (read/write/manage groups)
- [x] Edit scopes, revoke, copy `mcp.json` snippet

### H5 — Audit

- [x] `integration_token.created`, `scopes_updated`, `scopes_truncated`, `revoked`, `used`
- [x] `used` dedup 60s per `token_id+route`
- [x] Splunk-lite quick filters for integration token actions

## P6a — documentation + external server

| ID | Deliverable | Status |
|----|-------------|--------|
| A1 | Operator guide published | ✅ [mcp-operator.md](../guides/mcp-operator.md) |
| A2 | `@gosite/mcp` separate repo — stdio | ⬜ external publish |
| A3 | Token-only `mcp.json` examples | ✅ in operator guide |
| A4 | Tool guidelines map to OpenAPI | ✅ MCP cmd maps MVP tools |

## P6b — official plugin

| ID | Deliverable | Status |
|----|-------------|--------|
| B1 | `plugins/_templates/tier1-mcp/` | ✅ |
| B2 | Catalog artifact `gosite/mcp` (`plugins/gosite/mcp/`) | ✅ build via Makefile |
| B3 | Dynamic `tools/list` from introspection | ✅ |
| B4 | MVP tools: system, websites, nginx, docker, jobs, plugins | ✅ |
| B5 | `integration_token.used` audit with route | ✅ |
| B6 | Tests: stdio tools + token service tests | ✅ |

## P6c — HTTP remote (blocked)

Prerequisites unchanged — third listener sequence required.
