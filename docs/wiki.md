# GitHub Wiki — GoSite

How to publish bilingual docs to the GitHub wiki.

## Languages

| Pattern | Language | Wiki pages |
|---------|----------|------------|
| `docs/*.md` | English (primary) | `Architecture.md`, `Home.md`, … |
| `docs/*_id.md` | Indonesian | `Architecture-id.md`, `Home-id.md`, … |

See [LOCALIZATION.md](./LOCALIZATION.md).

## Export

```bash
make wiki-export
# output: docs/wiki-export/
# sidebars: _Sidebar.md (EN), _Sidebar-id.md (ID)
```

## Push to wiki

```bash
git clone https://github.com/jahrulnr/GoSite.wiki.git /tmp/gosite.wiki
cp docs/wiki-export/*.md /tmp/gosite.wiki/
cd /tmp/gosite.wiki && git add -A && git commit -m "docs: sync bilingual wiki" && git push
```

## Suggested wiki structure

| Wiki page | Source (EN) | Source (ID) |
|-----------|-------------|-------------|
| Home | README excerpt | docs/README_id.md |
| Architecture | architecture.md | architecture_id.md |
| Domain model | domain-model.md | domain-model_id.md |
| API reference | api-inventory.md | api-inventory_id.md |
| Nginx auto-repair | nginx-repair.md | nginx-repair_id.md |
| Sequences | sequences/*.md | sequences/*_id.md |

OpenAPI: [`api/openapi.yaml`](../api/openapi.yaml)
