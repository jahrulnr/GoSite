# GitHub Wiki — GoSite

How to publish bilingual docs to the GitHub wiki.

## Languages

| Pattern | Language | Wiki pages |
|---------|----------|------------|
| `docs/*.md` | English (primary) | `Architecture.md`, `Home.md`, … |
| `docs/*_id.md` | Indonesian | `Architecture-id.md`, `Home-id.md`, … |

See [LOCALIZATION.md](./localization.md).

## Export

```bash
make wiki-export
# output: docs/wiki-export/ (gitignored — generated artifact)
# sidebars: `_Sidebar.md` only (bilingual EN/ID links per row; GitHub Wiki has one global sidebar)
```

On push or pull request that touches `docs/`, [`.github/workflows/wiki.yml`](../.github/workflows/wiki.yml) runs `make wiki-export` and validates the output. **Publishing** to `GoSite.wiki` happens only when changes land on **`master`** (or `main`), or when you run the workflow manually with **publish** enabled.

**Wiki setup (once):**

1. Enable Wiki: repo **Settings → Features → Wikis**
2. Create one manual wiki page (e.g. `Home`) so the `GoSite.wiki` git repo exists
3. If `GITHUB_TOKEN` is rejected for wiki push, add repo secret **`WIKI_TOKEN`** (PAT with `repo` scope)

## Push to wiki (manual)

```bash
make wiki-export
bash scripts/push-wiki.sh   # needs GITHUB_TOKEN or WIKI_TOKEN
```

Or clone the wiki repo directly:

```bash
git clone https://github.com/jahrulnr/GoSite.wiki.git /tmp/gosite.wiki
cp docs/wiki-export/*.md /tmp/gosite.wiki/
cd /tmp/gosite.wiki && git add -A && git commit -m "docs: sync wiki bilingual" && git push
```

## Suggested wiki structure

| Wiki page | Source (EN) | Source (ID) |
|-----------|-------------|-------------|
| Home | README excerpt | docs/README_id.md |
| Architecture | architecture/overview.md | architecture/overview_id.md |
| Domain model | architecture/domain-model.md | architecture/domain-model_id.md |
| API reference | reference/api-inventory.md | reference/api-inventory_id.md |
| Nginx auto-repair | operations/nginx-repair.md | operations/nginx-repair_id.md |
| Sequences | sequences/*.md | sequences/*_id.md |
| Log search | guides/log-search.md | guides/log-search_id.md |
| Plugin installer | sequences/19-plugin-installer.md | sequences/19-plugin-installer_id.md |
| Plugin platform (ADR) | architecture/plugin-platform.md | _(EN; same page exported as Plugin-platform-id)_ |
| Plugin dev templates | [plugins/_templates/](../plugins/_templates/) | same repo path |

OpenAPI: [`api/openapi.yaml`](../api/openapi.yaml)
