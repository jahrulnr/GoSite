# Migration

Dokumen pendukung migrasi BangunSite → GoSite.

| File | Isi |
|------|-----|
| [backend-modules.md](./backend-modules.md) | Paket Go, fase implementasi, dependency |

## Status

| Area | Dokumentasi | Implementasi Go |
|------|-------------|-----------------|
| Sequence diagrams | ✅ 16 modul | ⬜ |
| API inventory | ✅ draft v1 | ⬜ |
| Domain model | ✅ | ⬜ |
| OpenAPI spec | ⬜ | ⬜ |

## Langkah disarankan

1. Review & revisi [api-inventory.md](../api-inventory.md)
2. Generate OpenAPI 3 dari inventory
3. Implement Fase 0 backend (auth + health)
