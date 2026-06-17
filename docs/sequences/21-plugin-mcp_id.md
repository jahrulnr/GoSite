# Sequence: Plugin MCP & token integrasi

Perluasan dari [19-plugin-installer_id.md](./19-plugin-installer_id.md) dan [20-plugin-remote-distribution_id.md](./20-plugin-remote-distribution_id.md).

**Status:** Desain — belum diimplementasi

Dokumen EN lengkap: [21-plugin-mcp.md](./21-plugin-mcp.md).

## Runtime

Panel dan MCP memanggil **`gosite serve :8080`** langsung. Nginx `:80`/`:443` hanya **website edge** — bukan jalur auth panel/MCP. Lihat [overview.md](../architecture/overview.md).

## Masalah

Operator ingin klien AI (Cursor, Claude, OpenClaw) memanggil GoSite secara imperatif tanpa panel. Menyimpan **email/password panel** di `mcp.json` tidak aman: sesi penuh, revoke lemah, tanpa scope per tool.

## Tujuan

- Plugin katalog **`gosite/mcp`** (Tier 1) + opsional server stdio komunitas.
- Auth mesin: Basic Auth panel (Gin, opsional) + **token akses** `gs_pat_*` dari UI plugin.
- **Alur operator:** install plugin → generate token → **pilih whitelist scope** → salin ke AI → `tools/list` dinamis.
- Lifecycle token: **generate**, **edit scope**, **revoke** (banyak token per plugin).

## Alur operator (dikunci)

```text
1. Install gosite/mcp + ack permissions manifest
2. Enable plugin
3. Generate access token (label, expiry opsional)
4. Pilih scope whitelist (subset manifest)
5. (Nanti) Edit whitelist pada token yang ada
6. Salin gs_pat_* sekali → mcp.json
7. Install MCP di klien AI → tools/list sesuai scope token
```

**Manifest `permissions`** = plafon keras. Scope token ⊆ manifest.

## Auth

| Klien | L1 | L2 |
|-------|----|----|
| Browser | Basic panel (Gin, opsional) | Cookie session |
| MCP | Basic panel (opsional) | Header `X-Gosite-Access-Token: gs_pat_…` |

**Ditolak produksi:** password panel di env MCP.

## API token (P6-host-auth)

| Method | Path | Fungsi |
|--------|------|--------|
| `POST` | `/api/v1/plugins/{id}/integration-tokens` | Buat token + pilih scope |
| `GET` | `/api/v1/plugins/{id}/integration-tokens` | Daftar (tanpa secret) |
| `PATCH` | `…/integration-tokens/{tokenId}` | **Edit whitelist scope** |
| `DELETE` | `…/integration-tokens/{tokenId}` | Revoke |
| `GET` | `/api/v1/integration-tokens/self` | Introspeksi scope (dari MCP) |

## Scope ↔ tool MCP

| Scope | Tool |
|-------|------|
| `system:read` | `system` |
| `websites:read` / `websites:write` | `websites` |
| `nginx:read` | `nginx` |
| `nginx:manage` | `nginx` (reload) |
| `docker:manage` | `docker` |
| `jobs:read` | `jobs` |
| `plugins:read` | `plugins` |

**`tools/list` dinamis** — agent hanya melihat tool yang scope-nya ada di whitelist token.

## Contoh mcp.json

```json
{
  "mcpServers": {
    "gosite": {
      "command": "npx",
      "args": ["-y", "@gosite/mcp"],
      "env": {
        "GOSITE_URL": "https://panel.example.com:8080",
        "GOSITE_BASIC_USER": "admin",
        "GOSITE_BASIC_PASS": "admin",
        "GOSITE_ACCESS_TOKEN": "gs_pat_..."
      }
    }
  }
}
```

## Gelombang implementasi

```text
P6-host-auth  →  DB token, middleware, API, UI generate/edit/revoke
P6a           →  dokumentasi + template eksternal stdio
P6b           →  plugin resmi gosite/mcp + tier1-mcp template
P6c           →  HTTP MCP remote (TLS, allowlist) — nanti
```

## Di luar scope gelombang 1

- Route Gin `/mcp` di core
- OAuth MCP
- WASM tier 2
- Password panel di env MCP (produksi)

## Referensi

- [overview.md](../architecture/overview.md)
- [plugin-platform.md](../architecture/plugin-platform.md)
- [03-authentication.md](./03-authentication.md)
