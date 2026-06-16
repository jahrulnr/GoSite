# Documentation languages

GoSite docs and wiki are **bilingual**:

| File pattern | Language | Role |
|--------------|----------|------|
| `*.md` | **English** | Primary — default links and wiki pages without suffix |
| `*_id.md` | **Indonesian** | Secondary — same structure, Indonesian prose |

## Conventions

- Cross-links in English files point to `*.md`.
- Cross-links in Indonesian files point to `*_id.md`.
- Code, paths, API routes, and Mermaid diagrams stay identical in both languages.
- Wiki export: English → `Architecture.md`, Indonesian → `Architecture-id.md`.

## Commands

```bash
make wiki-export   # builds docs/wiki-export/ (EN + ID pages)
```

## Adding a new page

1. Write `topic.md` in English.
2. Add `topic_id.md` in Indonesian.
3. Run `make wiki-export` and push the wiki repo if needed.
