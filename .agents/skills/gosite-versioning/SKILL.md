---
name: gosite-versioning
description: GoSite and plugin SemVer policy — minGoSiteVersion, -dev local vs release builds, host/plugin bump rules, build/deploy version coupling. Use when bumping gosite or plugin versions, setting minGoSiteVersion, releasing Docker images, bundled plugins, or assessing compatibility between host and plugins.
---

# GoSite versioning

## Policy (operator rules)

1. plugin perlu punya min_version compatible
2. setiap gosite atau plugin update, perlu di set version
3. version terdiri dari 3 level: [breaking changes].[minor changes/new feature].[patch/optimization/bug fix]

## `-dev` suffix (local vs release)

| Build source | Version form | Example |
|--------------|--------------|---------|
| **Local** (`make build`, `go run`, `make dev-api`) | `X.Y.Z-dev` | `1.0.0-dev` |
| **GitHub release / production image** | `X.Y.Z` (no suffix) | `1.0.0` |

`-dev` marks artifacts **not** from a tagged release — local compile, dirty tree, or CI smoke without release tag.

Rules:

- **Never** publish `-dev` to GitHub release assets or production `PROD_IMAGE`.
- **Compatibility compare** strips `-dev` before numeric check — `1.0.0-dev` satisfies `minGoSiteVersion: 1.0.0`.
- **UI / provenance** keeps full string so operators see local vs release (`source_ref: gosite@1.0.0-dev` vs `gosite@1.0.0`).
- **Plugin zip (local)** — set `manifest.version` to `A.B.C-dev` when building locally; release zip uses `A.B.C`.

Host local build: `Makefile` sets `VERSION=$(BASE_VERSION)-dev` unless `RELEASE=1`.

```bash
make build              # → 1.0.0-dev (from latest v* tag)
make build RELEASE=1    # → 1.0.0 (release-shaped)
docker build --build-arg VERSION=1.0.0 ...   # production — no -dev
```

## Two version streams

| Stream | Field | Who sets it | Meaning |
|--------|-------|-------------|---------|
| **Host** | `buildinfo.Version` / `APP_VERSION` | GoSite release | SemVer of the control plane |
| **Plugin artifact** | `manifest.version` | Plugin release | SemVer of that zip |
| **Compatibility floor** | `manifest.minGoSiteVersion` | Plugin author | Lowest host version the plugin supports |

Host checks **plugin → host** at install/enable: `minGoSiteVersion <= hostVersion` (numeric `X.Y.Z`; `-dev` stripped before compare).

Host does **not** today enforce **host → plugin** (no `maxPluginVersion` / `minPluginVersion` on host).

Separate contracts (bump only on breaking platform changes):

| Field | Breaking when |
|-------|----------------|
| `apiVersion` | Plugin manifest / lifecycle schema (`gosite-plugin/1`) |
| `rpcVersion` | Tier-1 go-plugin RPC (`1`) |
| `configVersion` | Plugin config JSON schema (switch runs `MigrateConfig`) |

## SemVer bump rules

**MAJOR** — breaking: removed/changed hooks, permission semantics, RPC/manifest contract, nginx API used by plugins, DB schema plugins depend on.

**MINOR** — backward compatible: new hooks (optional), new API fields, new permissions (additive), new UI routes.

**PATCH** — bugfix/perf/docs inside same compatibility surface; no manifest contract change.

Bump `X.Y.Z` only on the numeric triple; re-append `-dev` for local host/plugin builds after bump.

### Host release

Bump `MAJOR.MINOR.PATCH` in git tag / `PROD_IMAGE` / Docker `VERSION` build-arg together (**no `-dev`**).

Touch: `internal/buildinfo/buildinfo.go` (default dev string), `Makefile` `BASE_VERSION` / `RELEASE=1`, `scripts/deploy-vm.example.sh` `PROD_IMAGE`, CI `build-args.VERSION`, optional `APP_VERSION` in compose.

### Plugin release

Bump `manifest.version` in `plugins/<vendor>/<name>/manifest.json` (rebuild zip). Release: `A.B.C`; local zip: `A.B.C-dev`.

Set `minGoSiteVersion` to the **lowest release host version** (`X.Y.Z`, no `-dev`) that can run this artifact.

Official bundled `gosite/*`: built in same image as host; `minGoSiteVersion` still documents community/remote installs. Bundled seed skips the check (same build).

## Release checklist

```
Host release X.Y.Z (no -dev)
- [ ] git tag vX.Y.Z-rc.1 (or rc.N) after smoke QA
- [ ] Docker build --build-arg VERSION=X.Y.Z
- [ ] bundled-plugins rebuilt in image
- [ ] deploy PROD_IMAGE=gosite:X.Y.Z
- [ ] verify: gosite plugin list → gosite/mcp@* installed
- [ ] release QA: gosite-release-qa full on rc image → PASS (see docs/qa/release-matrix.md)
- [ ] tag vX.Y.Z after full QA PASS; optional qa-layperson-ui-ux

Host local smoke
- [ ] make build → buildinfo  X.Y.Z-dev
- [ ] panel shows version with -dev suffix
- [ ] release QA: gosite-release-qa smoke on make dev-api (optional before PR)

Plugin release A.B.C (no -dev)
- [ ] manifest.version = A.B.C
- [ ] minGoSiteVersion = lowest compatible host release (no -dev)
- [ ] make -C plugins/... build RELEASE=1 (or hand-edit manifest)
- [ ] sign + gosite.plugin.json releases[]

Plugin local zip
- [ ] manifest.version = A.B.C-dev
- [ ] minGoSiteVersion unchanged (release floor)
```

## Agent workflow

1. **Classify change** — host-only, plugin-only, or both; breaking vs additive; local vs release artifact.
2. **Pick bump level** per SemVer table; apply `-dev` only for local builds.
3. **Update coupled identifiers** — see [reference.md](reference.md).
4. **Align minGoSiteVersion** to release host version (never `-dev` in floor).
5. **Verify** — `go test ./internal/service/plugin/... ./internal/buildinfo/...`; install smoke.

## Common mistakes

| Mistake | Symptom | Fix |
|---------|---------|-----|
| Ship `X.Y.Z-dev` in GitHub release | Operators cannot tell release from dev | Strip `-dev`; tag `vX.Y.Z` |
| Docker `VERSION=dev` or git SHA | Compare treats host as `0.0.0` | Pass SemVer build-arg |
| `minGoSiteVersion: 1.0.0-dev` | Non-standard floor | Use `1.0.0` (release) as floor |
| Local plugin zip without `-dev` | Looks like release artifact | Use `A.B.C-dev` in manifest |

## Related skills & docs

- Release regression QA: [gosite-release-qa](../gosite-release-qa/SKILL.md) + `docs/qa/release-matrix.md`
- Plugin build/sign: [gosite-plugin-dev](../gosite-plugin-dev/SKILL.md)
- Bundled Path C: `docs/sequences/23-builtin-plugins.md`
- Install contract: `docs/sequences/19-plugin-installer.md`
- File matrix: [reference.md](reference.md)
