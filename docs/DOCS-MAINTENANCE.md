# Documentation maintenance

How to keep `docs/` aligned with the codebase during active development.

**Last audit:** 2026-06-17 (seq 22 nginx metrics SA-8 shipped)

## Layer model

| Layer | Path | Audience | Update when |
|-------|------|----------|-------------|
| **Contract** | `api/openapi.yaml` | API consumers, codegen | New/changed HTTP routes |
| **Architecture** | `docs/architecture/` | Contributors | New backend modules, runtime changes, ADRs |
| **Reference** | `docs/reference/` | API mapping | Legacy Laravel map, endpoint summaries, integration tokens, MCP tools |
| **Operations** | `docs/operations/` | Operators, contributors | Runtime behaviour (nginx repair, etc.) |
| **Guides** | `docs/guides/` | Contributors, wiki maintainers, operators | Dev setup, localization, wiki export, MCP operator setup |
| **Sequences** | `docs/sequences/*.md` | Feature design + review | New feature or behaviour change |
| **Implementation tracker** | `docs/implementation/`, `*-impl.md` | Agents / implementers | Wave start/complete |
| **Migration map** | `docs/migration/` | BangunSite → GoSite | Legacy mapping only; not for new features |
| **Wiki export** | `docs/wiki-export/` (gitignored) | End users | After user-facing doc changes → `make wiki-export` |
| **Plugin templates** | `plugins/_templates/docs/` | Plugin authors | Manifest/hook contract changes |

**Source of truth order:** code → OpenAPI → sequences → architecture → wiki export.

## Known drift (v1.3.1 audit)

| Item | Issue | Action |
|------|-------|--------|
| Sequence 20 header | Said "Proposed" while wave G shipped | ✅ Fixed — status **Implemented** |
| `sequences/README` | Seq 20 marked 📋 Proposed | ✅ Fixed |
| `architecture/plugin-platform.md` | Remote distribution listed under P4+ deferred | ✅ Fixed — P4 remote install |
| `architecture/overview.md` | No `plugin` module row | ✅ Fixed |
| `reference/api-inventory.md` | No Plugins section | ✅ Fixed |
| `implementation/` | Only WAVE-SA-1..7; no plugin waves | ✅ Added WAVE-PLUGIN + G tracker link |
| `api/openapi.yaml` | Wave G plugin routes added (v1.3.1 doc pass) | ✅ |
| `20-plugin-remote-distribution.md` body | Long spec still reads like a proposal in places | OK as design doc; impl truth in `*-impl.md` |
| Wiki `Plugin-platform` / `Plugin-installer` pages | May lag until `make wiki-export` on `master` | Run export after doc PR merges |
| `README_id.md` module diagram | Still 14 areas, no Plugins/Terminal | ✅ Fixed in README_id |
| Core architecture docs | Traffic model: panel `:8080` ∥ nginx `:80/:443` (gosite controls nginx, does not proxy websites) | ✅ Fixed |
| Seq 21 MCP design | Monolithic `21-plugin-mcp.md` | ✅ Split — index + layer docs + `WAVE-PLUGIN-P6` |
| Seq 22 nginx metrics | stub_status + VTS not documented | ✅ seq 22 docs + `WAVE-SA-8` + `api/openapi.yaml` nginx metrics paths |

## Checklist — ship a feature

1. **Sequence** — add or update `docs/sequences/NN-*.md` (+ `_id` stub if bilingual wiki)
2. **Layer docs** — update the doc in the right layer (architecture / reference / guides / operations), not only the sequence index
3. **Impl tracker** — `*-impl.md` or `docs/implementation/WAVE-*.md` with checkboxes
3. **OpenAPI** — paths, schemas, examples under `api/`
4. **Architecture** — module table in `architecture/overview.md`; ADR in `architecture/` if architectural
5. **API inventory** — one summary section (legacy map is optional for greenfield endpoints)
6. **Index** — `docs/sequences/README.md` status column
7. **Root README** — `docs/README.md` document status table if major area added
8. **Wiki** — `make wiki-export` before release tag (CI publishes from `master`)

## Checklist — release tag

1. `git log v<prev>..HEAD --oneline` → release notes (group by theme)
2. Confirm sequence index statuses match merged work
3. Run `make wiki-export`; spot-check `docs/wiki-export/Home.md` links
4. Annotated tag message lists user-visible changes (see `git-workflow-rules` skill)

## File conventions

- **EN canonical** for long specs; `*_id.md` = Indonesian summary + link to EN
- **Status line** at top: `**Status:** Implemented | Proposed | Deprecated`
- **Impl companion:** `NN-feature-impl.md` for agent-oriented checklists (not wiki-exported by default)
- Do not duplicate full API schemas in markdown — link to `api/openapi.yaml`

## Related

- [sequences/README.md](./sequences/README.md) — feature index
- [implementation/README.md](./implementation/README.md) — wave index
- [guides/wiki.md](./guides/wiki.md) — GitHub wiki export
