# GoSite Documentation

Runtime, API, and migration docs for **GoSite** (Go + Preact hosting panel).

> **Languages:** English (this file) · [Bahasa Indonesia](./README_id.md)  
> **GitHub wiki:** [guides/wiki.md](./guides/wiki.md) · [guides/wiki_id.md](./guides/wiki_id.md)  
> **Convention:** [guides/localization.md](./guides/localization.md)

## Document status

| Category | Status |
|----------|--------|
| Architecture & domain model | Aligned v1.3.1 — see [DOCS-MAINTENANCE.md](./DOCS-MAINTENANCE.md) |
| Sequences 01–22 + nginx-repair | Updated for GoSite (seq 22 = nginx metrics SA-8) |
| Sequence 21 (MCP) | Split across architecture / reference / guides — see [21-plugin-mcp.md](./sequences/21-plugin-mcp.md#document-map) |
| Plugin templates | `plugins/_templates/` (tier 0–3 scaffolds) |
| `api/openapi.yaml` | Canonical API contract (plugin wave G included) |
| `migration/` | Legacy BangunSite reference (not updated per feature) |
| `implementation/` | WAVE-SA-1..8 + [WAVE-PLUGIN-G](./implementation/WAVE-PLUGIN-G.md) + [WAVE-PLUGIN-P6](./implementation/WAVE-PLUGIN-P6.md) |

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

| Folder / file | Description |
|---------------|-------------|
| [architecture/](./architecture/) | Runtime overview, domain model, plugin ADR |
| [reference/](./reference/) | API inventory, [integration tokens](./reference/integration-tokens.md), [MCP tools](./reference/mcp-tools.md) |
| [operations/](./operations/) | Nginx auto-repair |
| [guides/](./guides/) | Dev setup, wiki export, localization, [MCP operator](./guides/mcp-operator.md) |
| [sequences/](./sequences/) | Mermaid flow diagrams per feature |
| [implementation/](./implementation/) | Implementation wave trackers (SA + plugin) |
| [migration/](./migration/) | BangunSite migration notes |
| [plugins/_templates/](../plugins/_templates/) | Plugin development scaffolds |
| [DOCS-MAINTENANCE.md](./DOCS-MAINTENANCE.md) | Doc layers, drift audit, release checklist |

## Wiki export

```bash
make wiki-export   # → docs/wiki-export/ (gitignored; EN + *-id pages)
```

## Build Docker on restricted DNS networks

Some ISPs block public resolvers. `make build-docker` uses `--network=host` so image pulls use the host resolver. See [README](../README.md) for details.
