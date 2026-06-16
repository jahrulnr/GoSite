# Migration

Migration support documents for BangunSite → GoSite.

| File | Contents |
|------|----------|
| [backend-modules.md](./backend-modules.md) | Go packages, implementation phases, dependencies |

## Status

| Area | Documentation | Go implementation |
|------|---------------|-------------------|
| Sequence diagrams | ✅ 01–20 | ✅ |
| API inventory + OpenAPI | ✅ wave G plugin routes | ✅ handlers |
| Domain model | ✅ | ✅ |
| Nginx auto-repair | ✅ | ✅ |
| Certbot + SSE | ✅ | ✅ |
| Plugin platform + remote install | ✅ seq 19–20 | ✅ v1.3.1 |

New GoSite features after BangunSite cutover are documented in **sequences/** and **architecture/** — not in `migration/backend-modules.md` phase tables.

## Suggested steps

1. Deploy the latest image (`make up`) and verify the production stack
2. Review [api-inventory.md](../api-inventory.md) and `api/openapi.yaml` against handlers
3. After `docs/` changes: [DOCS-MAINTENANCE.md](../DOCS-MAINTENANCE.md) checklist → `make wiki-export`
