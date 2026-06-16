# GoSite Documentation

Runtime, API, and migration docs for **GoSite** (Go + Preact hosting panel).

> **Languages:** English (this file) · [Bahasa Indonesia](./README_id.md)  
> **GitHub wiki:** [wiki.md](./wiki.md) · [wiki_id.md](./wiki_id.md)  
> **Convention:** [LOCALIZATION.md](./LOCALIZATION.md)

## Document status

| Category | Status |
|----------|--------|
| Architecture & domain model | Aligned with implementation |
| All sequences (01–19) + nginx-repair | Updated for GoSite |
| Plugin templates | `plugins/_templates/` (tier 0–3 scaffolds) |
| `api/openapi.yaml` | Canonical API contract |
| `migration/` | Legacy reference + module map |

## Source of truth

| Item | Location |
|------|----------|
| Repository | `jahrulnr/GoSite` |
| OpenAPI | `api/openapi.yaml` |
| Go backend | `internal/` |
| Frontend | `web/` |
| Nginx / webconfig templates | `config/nginx`, `config/webconfig` |
| Production data | `/storage` volume |

## Document map

| Document | Description |
|----------|-------------|
| [architecture.md](./architecture.md) | Container runtime, Go modules, persistent paths |
| [domain-model.md](./domain-model.md) | SQLite entities & filesystem artifacts |
| [api-inventory.md](./api-inventory.md) | REST API map |
| [nginx-repair.md](./nginx-repair.md) | `nginx -t` fallback + auto-fix |
| [wiki.md](./wiki.md) | GitHub wiki export guide |
| [sequences/](./sequences/) | Mermaid flow diagrams per feature |
| [plugins/_templates/](../plugins/_templates/) | Plugin development scaffolds |
| [migration/](./migration/) | BangunSite migration notes |

## Wiki export

```bash
make wiki-export   # → docs/wiki-export/ (gitignored; EN + *-id pages)
```

## Build Docker on restricted DNS networks

Some ISPs block public resolvers. `make build-docker` uses `--network=host` so image pulls use the host resolver. See [README](../README.md) for details.
