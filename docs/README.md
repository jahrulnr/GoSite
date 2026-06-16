# GoSite Documentation

Runtime, API, and migration docs for **GoSite** (Go + Preact hosting panel).

> **Languages:** English (this file) · [Bahasa Indonesia](./README_id.md)  
> **GitHub wiki:** [wiki.md](./wiki.md) · [wiki_id.md](./wiki_id.md)  
> **Convention:** [LOCALIZATION.md](./LOCALIZATION.md)

## Document status

| Category | Status |
|----------|--------|
| Architecture & domain model | Mostly aligned — see [DOCS-MAINTENANCE.md](./DOCS-MAINTENANCE.md) |
| Sequences 01–20 + nginx-repair | Updated for GoSite (seq 20 = wave G, v1.3.1) |
| Plugin templates | `plugins/_templates/` (tier 0–3 scaffolds) |
| `api/openapi.yaml` | Canonical API contract (plugin wave G included) |
| `migration/` | Legacy BangunSite reference (not updated per feature) |
| `implementation/` | WAVE-SA-1..7 + [WAVE-PLUGIN-G](./implementation/WAVE-PLUGIN-G.md) |

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
| [architecture/plugin-platform.md](./architecture/plugin-platform.md) | Plugin ADR (tier 0–3, hooks, remote install) |
| [DOCS-MAINTENANCE.md](./DOCS-MAINTENANCE.md) | Doc layers, drift audit, ship/release checklist |
| [domain-model.md](./domain-model.md) | SQLite entities & filesystem artifacts |
| [api-inventory.md](./api-inventory.md) | REST API map |
| [nginx-repair.md](./nginx-repair.md) | `nginx -t` fallback + auto-fix |
| [wiki.md](./wiki.md) | GitHub wiki export guide |
| [sequences/](./sequences/) | Mermaid flow diagrams per feature |
| [plugins/_templates/](../plugins/_templates/) | Plugin development scaffolds |
| [migration/](./migration/) | BangunSite migration notes |
| [implementation/](./implementation/) | Implementation wave trackers (SA + plugin) |

## Wiki export

```bash
make wiki-export   # → docs/wiki-export/ (gitignored; EN + *-id pages)
```

## Build Docker on restricted DNS networks

Some ISPs block public resolvers. `make build-docker` uses `--network=host` so image pulls use the host resolver. See [README](../README.md) for details.
