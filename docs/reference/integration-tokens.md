# Integration access tokens (`gs_pat_*`)

Host-issued scoped tokens for MCP and future tier-0 webhooks. Canonical API contract: `api/openapi.yaml` (paths added in P6-host-auth).

**Status:** Design — not implemented

**Auth model:** [plugin-integration-auth.md](../architecture/plugin-integration-auth.md) · **Sequence:** [21-plugin-mcp.md](../sequences/21-plugin-mcp.md)

## API

Session required for admin CRUD. Access token required for introspection.

When `AUTH_ENABLE=true`, **all** `/api/v1` endpoints (including `/integration-tokens/self`) also require `Authorization: Basic` — same Gin middleware as panel traffic.

| Method | Path | Auth | Body / response |
|--------|------|------|-----------------|
| `POST` | `/api/v1/plugins/{id}/integration-tokens` | Session | `{ "label", "scopes": [], "expires_at"? }` → `{ "token": "gs_pat_…" }` once |
| `GET` | `/api/v1/plugins/{id}/integration-tokens` | Session | List metadata + scopes (no secrets) |
| `PATCH` | `/api/v1/plugins/{id}/integration-tokens/{tokenId}` | Session | `{ "scopes": [] }` — edit whitelist |
| `DELETE` | `/api/v1/plugins/{id}/integration-tokens/{tokenId}` | Session | Revoke |
| `GET` | `/api/v1/integration-tokens/self` | Access token + Basic† | `{ "plugin_id", "scopes", "expires_at", "label" }` |

† Basic Auth when `AUTH_ENABLE=true`.

Validation: `scopes[]` ⊆ manifest `permissions` for the **currently enabled** plugin version on create and patch.

Token CRUD is **session-only** — not delegable to `gs_pat_*`. Scope strings: [plugin-permissions.md](./plugin-permissions.md).

## Database

Table `plugin_access_tokens` (migration in P6-host-auth H1):

| Column | Notes |
|--------|-------|
| `id` | UUID |
| `plugin_id` | FK — stable `vendor/name` identity (e.g. `gosite/mcp`); survives version switch |
| `created_under_version` | Semver string at create time (audit only; not FK) |
| `label` | Operator-defined |
| `token_hash` | SHA-256 |
| `scopes_json` | JSON array |
| `created_by_user_id` | FK users |
| `created_at`, `expires_at`, `revoked_at` | |
| `last_used_at` | Updated on **every** successful authenticated request (no dedup). Dedup 60s applies to audit log entries only — see Audit events. |

Also documented in [domain-model.md](../architecture/domain-model.md).

### Token lifecycle policy (locked)

| Event | Token behavior |
|-------|----------------|
| Plugin **disabled** | **Suspended** — middleware returns 401; `revoked_at` unchanged; resumes when plugin re-enabled |
| Plugin **uninstalled** | **Hard-revoked** — set `revoked_at`; cannot be used again |
| Plugin **version switch** | Tokens **survive** — scopes reconciled against new manifest (see below) |

### Version switch reconciliation

Seq 19 `POST /api/v1/plugins/{vendor}/{name}/switch` can change the enabled manifest.

**Timing (locked):** reconciliation runs **only after** the switch is fully committed — `enabled` state written for vNext and transaction complete. **Never** during `enabling` / `disabling` transitions or before `StartRuntime(vNext)` succeeds. If switch fails (e.g. `enable_failed` rollback), reconciliation must not run — tokens keep their current scopes.

After a **successful** switch:

```text
for each active token where plugin_id matches:
  if token.scopes ⊄ new_manifest.permissions:
    auto-truncate scopes to intersection
    if truncated scopes empty:
      hard-revoke token
      emit integration_token.scopes_truncated then integration_token.revoked + notify operator
    else:
      emit integration_token.scopes_truncated + notify operator
```

Middleware always validates scopes against the **currently enabled** version's manifest — not the version at token creation.

## Plugin UI (gosite/mcp)

Route: `/plugins/gosite/mcp/integration` (host-rendered; data from plugin UI schema).

| Action | UX |
|--------|-----|
| **Generate** | Label + expiry; multi-select scopes grouped read / write / manage; default pre-check read-only only |
| **Edit scopes** | Same picker on existing row; save → PATCH |
| **Revoke** | Confirm → immediate 401 |
| **Copy setup** | One-time token + sample `mcp.json` snippet — [mcp-operator.md](../guides/mcp-operator.md) |

## Audit events

- `integration_token.created`, `integration_token.scopes_updated`, `integration_token.scopes_truncated`, `integration_token.revoked`
- `integration_token.used` — token `id` (UUID, not label), route, correlation id, client IP (never the secret). Per-call with **dedup window 60s** per `token_id+route` to limit volume under heavy MCP usage.

Splunk-lite queryable (P6-host-auth H5).

## Implementation tracker

Checkboxes and gate status: [21-plugin-mcp-impl.md](../sequences/21-plugin-mcp-impl.md) · [WAVE-PLUGIN-P6.md](../implementation/WAVE-PLUGIN-P6.md).
