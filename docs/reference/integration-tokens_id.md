# Token akses integrasi (`gs_pat_*`)

Token scoped dari host untuk MCP. Kontrak API: `api/openapi.yaml` (ditambah di P6-host-auth).

**Status:** Desain — belum diimplementasi

**Auth:** [plugin-integration-auth.md](../architecture/plugin-integration-auth.md) · **Sequence:** [21-plugin-mcp.md](../sequences/21-plugin-mcp.md)

## API (ringkas)

| Method | Path | Auth |
|--------|------|------|
| POST/GET/PATCH/DELETE | `/api/v1/plugins/{id}/integration-tokens` | Session |
| GET | `/api/v1/integration-tokens/self` | Access token + Basic† |

† Basic saat `AUTH_ENABLE=true`.

## Database

Tabel `plugin_access_tokens` — FK `plugin_id` (`vendor/name`), bukan `plugin_version_id`.

**Lifecycle:** disable = suspend; uninstall = hard-revoke; switch = rekonsiliasi scope.

Detail lengkap: [integration-tokens.md](./integration-tokens.md) (EN).
