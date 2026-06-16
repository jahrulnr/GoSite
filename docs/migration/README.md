# Migration

Migration support documents for BangunSite → GoSite.

| File | Contents |
|------|----------|
| [backend-modules.md](./backend-modules.md) | Go packages, implementation phases, dependencies |

## Status

| Area | Documentation | Go implementation |
|------|---------------|-------------------|
| Sequence diagrams | ✅ 18 modules | ✅ |
| API inventory + OpenAPI | ✅ | ✅ |
| Domain model | ✅ | ✅ |
| Nginx auto-repair | ✅ | ✅ |
| Certbot + SSE | ✅ | ✅ |

## Suggested steps

1. Deploy the latest image (`make up`) and verify the production stack
2. Review [api-inventory.md](../api-inventory.md) against handler implementation
3. Update the wiki via `make wiki-export` after `docs/` changes
