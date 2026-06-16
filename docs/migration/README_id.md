# Migration

Dokumen pendukung migrasi BangunSite → GoSite.

| File | Isi |
|------|-----|
| [backend-modules.md](./backend-modules.md) | Paket Go, fase implementasi, dependency |

## Status

| Area | Dokumentasi | Implementasi Go |
|------|-------------|-----------------|
| Sequence diagrams | ✅ 18 modul | ✅ |
| API inventory + OpenAPI | ✅ | ✅ |
| Domain model | ✅ | ✅ |
| Nginx auto-repair | ✅ | ✅ |
| Certbot + SSE | ✅ | ✅ |

## Langkah disarankan

1. Deploy image terbaru (`make up`) dan verifikasi stack produksi
2. Review [api-inventory.md](../api-inventory_id.md) vs implementasi handler
3. Perbarui wiki via `make wiki-export` setelah perubahan `docs/`
