# GoSite API Contract (v1)

Single source of truth for the panel REST API. Contract tests are derived from this spec.

## Base URL

| Environment | URL |
|-------------|-----|
| Production | `https://<host>:8080/api/v1` |
| Local dev | `https://localhost:8080/api/v1` |

## Authentication

1. **Session cookie** — `gosite_session` (HTTP-only). Issued by `POST /auth/login`.
2. **HTTP Basic** (optional) — When `AUTH_ENABLE=true`, all `/api/v1/*` routes require Basic Auth before session checks.

Protected routes return `401` with:

```json
{ "error": { "code": "SESSION_EXPIRED", "message": "..." } }
```

## Error format

All errors use:

```json
{
  "error": {
    "code": "VALIDATION_FAILED",
    "message": "human-readable message"
  }
}
```

Stable codes are listed in [`pkg/apperror/codes.go`](../pkg/apperror/codes.go).

## Spec files

| File | Purpose |
|------|---------|
| [`openapi.yaml`](./openapi.yaml) | OpenAPI 3.1 path + schema definitions |
| [`examples/`](./examples/) | Golden JSON responses for contract tests |

## Contract verification

```bash
make contract-check
```

## Out of scope (v1)

- `PUT /settings/php`, `/settings/fpm`, `/settings/pool` (legacy Laravel editors)
- `GET /settings` aggregate (profile-only via `PUT /settings/profile` + `GET /auth/me`)

## Related docs

- [`docs/api-inventory.md`](../docs/api-inventory.md) — Laravel migration map (redirects here)
