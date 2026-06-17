# Plugin integration authentication

Machine auth for MCP clients and future tier-0 integrations. Extension of [03-authentication.md](../sequences/03-authentication.md).

**Status:** Implemented — P6-host-auth + P6b (2026-06-17)

**Sequence:** [21-plugin-mcp.md](../sequences/21-plugin-mcp.md) · **Token API:** [integration-tokens.md](../reference/integration-tokens.md)

## Runtime context

Panel users and MCP HTTP clients reach **`gosite serve :8080`** directly. Nginx `:80`/`:443` is website edge only — not in the panel/MCP auth path. See [overview.md](./overview.md).

## Auth layers

### Humans (unchanged)

| Layer | Mechanism | Credential |
|-------|-----------|------------|
| 1 — Panel API gate | `middleware.BasicAuth` on `gosite :8080` when `AUTH_ENABLE=true` | `AUTH_USER` / `AUTH_PASS` |
| 2 — Session | `middleware.RequireSession` | `gosite_session` cookie after login |

### Machines — MCP / integrations (new)

| Layer | Credential |
|-------|------------|
| 1 — Panel API gate | Same Gin Basic Auth when enabled |
| 2 — Integration | `X-Gosite-Access-Token: gs_pat_…` |

```http
Authorization: Basic <AUTH_USER:AUTH_PASS>   # when AUTH_ENABLE=true; Gin on :8080
X-Gosite-Access-Token: gs_pat_...
```

When `AUTH_ENABLE=false`, only the access token header is required for machine clients.

**Rejected for production:** `GOSITE_EMAIL` / `GOSITE_PASSWORD` in MCP env. Dev-only escape hatch: `GOSITE_INSECURE_SESSION=1` (undocumented in official examples). **Must be rejected** when `GOSITE_ENV=production` (or equivalent production flag) — startup fails if set.

## Token format

- Prefix: `gs_pat_` (GoSite Plugin Access Token)
- Entropy: ≥32 random bytes (base64url or hex)
- Storage: SHA-256 hash only; plaintext shown **once** at create
- Multiple tokens per plugin (labels: `cursor-laptop`, `ci-bot`, …); label uniqueness per `plugin_id` is optional — audit events always include token `id` (UUID)
- **Rotation:** revoke old token + create new (new UUID). No re-issue secret on the same row.

## Middleware

```text
RequireSessionOrAccessToken:
  if valid X-Gosite-Access-Token:
    load token → scopes, plugin_id, created_by
    require token not revoked and not expired
    require plugin enabled (disabled → 401; token suspended, not revoked)
    enforce scope for handler
  else:
    RequireSession (existing)
```

Scope strings and route mapping: [plugin-permissions.md](../reference/plugin-permissions.md).

## Security notes

| Threat | Mitigation |
|--------|------------|
| Token in `mcp.json` | Scoped + revocable + optional TTL; not a panel password |
| Stolen MCP subprocess | Minimum scopes; audit; revoke; prefer host RPC in P6b |
| Scope escalation via PATCH | Server validates ⊆ manifest permissions |
| Manifest shrink on version switch | Switch reconciliation auto-truncates or revokes stale scopes — [integration-tokens.md](../reference/integration-tokens.md#version-switch-reconciliation) |
| CSRF on token admin | Session-only mutations |
| Undeclared tool bypass | Dynamic `tools/list` + route scope enforcement — [mcp-tools.md](../reference/mcp-tools.md) |
| `GOSITE_INSECURE_SESSION` in prod | Rejected at startup when `GOSITE_ENV=production` |

## Related

- [integration-tokens.md](../reference/integration-tokens.md) — API, DB, lifecycle, audit
- [mcp-tools.md](../reference/mcp-tools.md) — tool registry, re-introspect
- [mcp-operator.md](../guides/mcp-operator.md) — operator `mcp.json` setup
- [plugin-permissions.md](../reference/plugin-permissions.md) — canonical scope registry
