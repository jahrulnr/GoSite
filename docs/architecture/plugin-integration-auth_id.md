# Autentikasi integrasi plugin

Auth mesin untuk klien MCP dan integrasi tier-0 mendatang. Perluasan [03-authentication.md](../sequences/03-authentication.md).

**Status:** Diimplementasi — P6-host-auth + P6b (2026-06-17)

**Sequence:** [21-plugin-mcp.md](../sequences/21-plugin-mcp.md) · **API token:** [integration-tokens.md](../reference/integration-tokens.md)

## Lapisan auth

| Klien | L1 | L2 |
|-------|----|----|
| Browser | Basic panel (Gin, opsional) | Cookie session |
| MCP | Basic panel (opsional) | `X-Gosite-Access-Token: gs_pat_…` |

`GOSITE_INSECURE_SESSION=1` hanya dev — **ditolak** jika `GOSITE_ENV=production`.

## Format token

- Prefix `gs_pat_`, hash SHA-256, plaintext sekali saat create
- Rotasi: revoke + buat baru (UUID baru)

## Middleware

`RequireSessionOrAccessToken` — validasi token, plugin enabled, enforce scope per-route; fallback ke session.

Detail lengkap: [plugin-integration-auth.md](./plugin-integration-auth.md) (EN).
