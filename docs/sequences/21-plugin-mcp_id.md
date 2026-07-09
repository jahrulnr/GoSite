# Sequence: Plugin MCP & token integrasi

Perluasan dari [19-plugin-installer_id.md](./19-plugin-installer_id.md) dan [20-plugin-remote-distribution_id.md](./20-plugin-remote-distribution_id.md).

**Status:** Desain — belum diimplementasi

**Keputusan dikunci:** 2026-06-17

Dokumen EN lengkap: [21-plugin-mcp.md](./21-plugin-mcp.md).

## Peta dokumen

| Topik | Lokasi |
|-------|--------|
| Index sequence | `sequences/21-plugin-mcp.md` |
| Tracker implementasi | `sequences/21-plugin-mcp-impl.md`, `implementation/WAVE-PLUGIN-P6.md` |
| Auth mesin | [plugin-integration-auth.md](../architecture/plugin-integration-auth.md) |
| API token, DB, lifecycle | [integration-tokens.md](../reference/integration-tokens.md) |
| Tool MCP & manifest | [mcp-tools.md](../reference/mcp-tools.md) |
| Panduan operator `mcp.json` | [mcp-operator.md](../guides/mcp-operator.md) |
| Registry scope | [plugin-permissions.md](../reference/plugin-permissions.md) |

## Ringkasan

- Plugin katalog **`gosite/mcp`** (Tier 1, monorepo) + `@gosite/mcp` stdio (repo terpisah).
- Auth mesin: Basic panel (opsional) + `gs_pat_*` scoped.
- Alur: install → enable → generate token + scope → salin ke AI client → `tools/list` dinamis.

## Gelombang

```text
P6-host-auth → P6a (dokumen + stdio) → P6b (plugin resmi) → P6c (HTTP, blocked)
```

## Keputusan (dikunci)

| Topik | Keputusan |
|-------|-----------|
| Rotasi token | Revoke + buat baru (UUID baru) |
| FK token | `plugin_id`, bukan `plugin_version_id` |
| Disable vs uninstall | Suspend vs hard-revoke |
| Rekonsiliasi switch | Setelah switch committed; auto-truncate; `scopes_truncated` → `revoked` jika kosong |
| Re-introspect | Lazy 403/401; retry hanya jika tool masih di registry |
| `last_used_at` | Update tiap call; dedup 60s hanya audit log |
| Artifact | Tier-1 monorepo; stdio komunitas repo terpisah |
| go-sdk | P6a experimental; P6b tunggu post-GA |

Detail: [21-plugin-mcp.md](./21-plugin-mcp.md) · [21-plugin-mcp-impl.md](./21-plugin-mcp-impl.md)
