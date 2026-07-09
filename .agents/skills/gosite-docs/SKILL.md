---
name: gosite-docs
description: Maintains GoSite bilingual documentation (docs/), wiki export, and release doc checklists. Use when editing docs, sequences, architecture/reference guides, running make wiki-export, polishing English prose, adding *_id.md pages, syncing GoSite.wiki, or when the user mentions documentation SOP, wiki, or localization.
---

# GoSite documentation SOP

Standard workflow for `docs/` in this repository. **Authoritative detail:** [docs/DOCS-MAINTENANCE.md](docs/DOCS-MAINTENANCE.md).

## Source-of-truth order

```
code → api/openapi.yaml → sequences → architecture/reference → wiki export
```

Do **not** duplicate full API schemas in markdown — link to `api/openapi.yaml`.

## Layer map (where to edit)

| Layer | Path | Update when |
|-------|------|-------------|
| Contract | `api/openapi.yaml` | HTTP routes change |
| Architecture | `docs/architecture/` | Modules, runtime, ADRs |
| Reference | `docs/reference/` | API inventory, tokens, MCP tools |
| Operations | `docs/operations/` | nginx repair, operator behaviour |
| Guides | `docs/guides/` | dev setup, wiki, localization, MCP operator |
| Sequences | `docs/sequences/` | Feature flows (Mermaid) |
| Implementation | `docs/implementation/`, `*-impl.md` | Wave trackers (not wiki by default) |
| Migration | `docs/migration/` | Legacy BangunSite only — not new features |

## Bilingual rules

| Pattern | Language | Wiki page |
|---------|----------|-----------|
| `*.md` | English (primary) | `Topic.md` |
| `*_id.md` | Indonesian | `Topic-id.md` |

- EN links → `*.md`; ID links → `*_id.md`
- Keep code, paths, API routes, and Mermaid **identical** across languages
- EN is canonical for long specs; `*_id.md` may be summary + link to EN

See [docs/guides/localization.md](docs/guides/localization.md).

## SOP — ship a feature

Copy and track:

```
- [ ] Sequence: docs/sequences/NN-feature.md (+ *_id.md if wiki-facing)
- [ ] Layer doc: architecture / reference / guides / operations (not index-only)
- [ ] Impl tracker: *-impl.md or docs/implementation/WAVE-*.md
- [ ] OpenAPI: api/openapi.yaml + examples
- [ ] architecture/overview.md module row (if new module)
- [ ] reference/api-inventory.md section (summary; legacy map optional)
- [ ] docs/sequences/README.md status column
- [ ] docs/README.md status table (major area only)
- [ ] make wiki-export — spot-check docs/wiki-export/Home.md
```

Add `**Status:** Implemented | Proposed | Deprecated` at top of sequence pages.

## SOP — new page

1. Write `path/topic.md` (English)
2. Add `path/topic_id.md` (Indonesian)
3. Cross-link using language rules above
4. If wiki-facing: register in `scripts/export-wiki.sh` if not auto-discovered; run `make wiki-export`
5. Do **not** commit `docs/wiki-export/` (gitignored artifact)

## SOP — polish English

1. Hand-write or edit EN `*.md` — do not rely on machine translation alone
2. Optional bulk assist (sequences/migration only):

```bash
python3 docs/scripts/id-to-en.py
```

3. Fix EN cross-links (`_id.md` → `.md` in English files)
4. Scan for leftover Indonesian in EN sources:

```bash
rg '\b(jika |untuk |dengan |dari |Validasi|Kelola|Lihat |Catatan|Peran|Fitur)\b' docs \
  --glob '*.md' --glob '!**/*_id.md' --glob '!**/wiki-export/**'
```

5. `make wiki-export`

## SOP — wiki export & publish

```bash
make wiki-export          # → docs/wiki-export/ (gitignored)
bash scripts/push-wiki.sh # manual; needs WIKI_TOKEN or GITHUB_TOKEN
```

- CI: `.github/workflows/wiki.yml` validates on docs PRs; **publishes wiki only from `master`**
- Wiki setup: [docs/guides/wiki.md](docs/guides/wiki.md)
- GitHub API cannot push `.wiki` repos — use `push-wiki.sh` or git clone

## SOP — release tag

1. `git log v<prev>..HEAD --oneline` → release notes by theme
2. Sequence index statuses match merged work
3. `make wiki-export`; spot-check Home + new pages
4. Annotated tag lists user-visible changes

## Writing quality

- Match tone and structure of neighbouring docs in the same layer
- Legacy BangunSite: keep in `<details>` or short migration sections — do not frame GoSite as Laravel
- Minimize scope: doc-only tasks get doc-only diffs
- Only `git commit` when the user explicitly asks

## Related files

| File | Purpose |
|------|---------|
| [docs/DOCS-MAINTENANCE.md](docs/DOCS-MAINTENANCE.md) | Drift audit, checklists |
| [scripts/export-wiki.sh](scripts/export-wiki.sh) | Bilingual wiki generator |
| [scripts/push-wiki.sh](scripts/push-wiki.sh) | Push wiki-export to GitHub |
| [docs/scripts/id-to-en.py](docs/scripts/id-to-en.py) | Phrase-replace ID → EN (assist only) |
