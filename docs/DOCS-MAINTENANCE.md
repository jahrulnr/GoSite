# Documentation maintenance

How to keep `docs/` aligned with the codebase during active development.

**Last audit:** 2026-06-17 (post **v1.3.1** — wave G remote plugin distribution)

## Layer model

| Layer | Path | Audience | Update when |
|-------|------|----------|-------------|
| **Contract** | `api/openapi.yaml` | API consumers, codegen | New/changed HTTP routes |
| **Architecture** | `docs/architecture.md`, `docs/architecture/` | Contributors | New backend modules, runtime changes |
| **Sequences** | `docs/sequences/*.md` | Feature design + review | New feature or behaviour change |
| **Implementation tracker** | `docs/implementation/`, `*-impl.md` | Agents / implementers | Wave start/complete |
| **Migration map** | `docs/migration/` | BangunSite → GoSite | Legacy mapping only; not for new features |
| **API inventory** | `docs/api-inventory.md` | Legacy Laravel → REST map | New REST areas (summary table) |
| **Wiki export** | `docs/wiki-export/` (gitignored) | End users | After user-facing doc changes → `make wiki-export` |
| **Plugin templates** | `plugins/_templates/docs/` | Plugin authors | Manifest/hook contract changes |

**Source of truth order:** code → OpenAPI → sequences → architecture → wiki export.

## Known drift (v1.3.1 audit)

| Item | Issue | Action |
|------|-------|--------|
| Sequence 20 header | Said "Proposed" while wave G shipped | ✅ Fixed — status **Implemented** |
| `sequences/README` | Seq 20 marked 📋 Proposed | ✅ Fixed |
| `architecture/plugin-platform.md` | Remote distribution listed under P4+ deferred | ✅ Fixed — P4 remote install |
| `architecture.md` | No `plugin` module row | ✅ Fixed |
| `api-inventory.md` | No Plugins section | ✅ Fixed |
| `implementation/` | Only WAVE-SA-1..7; no plugin waves | ✅ Added WAVE-PLUGIN + G tracker link |
| `api/openapi.yaml` | Wave G plugin routes added (v1.3.1 doc pass) | ✅ |
| `20-plugin-remote-distribution.md` body | Long spec still reads like a proposal in places | OK as design doc; impl truth in `*-impl.md` |
| Wiki `Plugin-platform` / `Plugin-installer` pages | May lag until `make wiki-export` on `master` | Run export after doc PR merges |
| `README_id.md` module diagram | Still 14 areas, no Plugins/Terminal | ✅ Fixed in README_id |
| Core architecture docs | `architecture*.md`, `domain-model*.md`, `api-inventory*.md`, `dev-mount-testing*`, `nginx-repair*` lagged v1.3.1 | ✅ Fixed this pass |

## Checklist — ship a feature

1. **Sequence** — add or update `docs/sequences/NN-*.md` (+ `_id` stub if bilingual wiki)
2. **Impl tracker** — `*-impl.md` or `docs/implementation/WAVE-*.md` with checkboxes
3. **OpenAPI** — paths, schemas, examples under `api/`
4. **Architecture** — module table in `architecture.md`; ADR in `architecture/` if architectural
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
- [wiki.md](./wiki.md) — GitHub wiki export
