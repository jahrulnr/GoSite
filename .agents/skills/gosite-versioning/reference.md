# GoSite versioning — reference

## `-dev` semantics

| Question | Answer |
|----------|--------|
| What is `-dev`? | Marker that binary/zip was built locally (or non-release pipeline), not downloaded from GitHub release. |
| Compatibility | Stripped in `semverParts()` — `1.0.0-dev` == `1.0.0` for `minGoSiteVersion` checks. |
| Display | Full string kept in `buildinfo.Version`, panel, `source_ref` provenance. |
| `buildinfo.IsDev()` | `true` when `Version` ends with `-dev`. |
| `minGoSiteVersion` | Always **release** form `X.Y.Z` (no `-dev`) — floor for published host lines. |
| `manifest.version` | Release zip: `A.B.C`; local `make build`: `A.B.C-dev`. |

## Makefile / Docker

| Command | VERSION |
|---------|---------|
| `make build` | `$(BASE_VERSION)-dev` |
| `make build RELEASE=1` | `$(BASE_VERSION)` |
| `docker build --build-arg VERSION=1.0.0` | `1.0.0` |
| `scripts/deploy.local.sh` | `PROD_VERSION` from `gosite:X.Y.Z` tag (no `-dev`) |
| `go run` without ldflags | `1.0.0-dev` (`buildinfo` default) |

`BASE_VERSION` = latest git tag matching `v*` (e.g. `v1.0.0` → `1.0.0`), else `1.0.0`.

## Assessment gaps

| Issue | Detail |
|-------|--------|
| CI non-SemVer | `.github/workflows/docker-build.yml` uses `VERSION=${{ github.sha }}` — not `-dev` nor release SemVer. |
| Plugin Makefile | Does not auto-stamp `manifest.version` with `-dev`; edit manifest or add `RELEASE=1` gate later. |
| One-way check | Plugin `minGoSiteVersion` only; no host max plugin version. |

## File touch matrix

### Host release `X.Y.Z` (no `-dev`)

| File | Action |
|------|--------|
| Git tag | `vX.Y.Z` |
| `Makefile` | `make build RELEASE=1` or Docker build-arg |
| `Dockerfile` | `ARG VERSION` → ldflags |
| `scripts/deploy-vm.example.sh` | `PROD_IMAGE=gosite:X.Y.Z` |
| `internal/buildinfo/buildinfo.go` | Dev default `1.0.0-dev` only |

### Host local `X.Y.Z-dev`

| File | Action |
|------|--------|
| `make build` | default `VERSION=$(BASE_VERSION)-dev` |
| `make dev-api` | `go run` inherits default or Makefile ldflags if wired |

### Plugin release `A.B.C` / local `A.B.C-dev`

| File | Action |
|------|--------|
| `plugins/.../manifest.json` | `version`, `minGoSiteVersion` (release floor) |
| Bundled / catalog | Official plugins follow host release in image |

## compareSemver behavior

```go
// "1.4.0-dev" → parts [1,4,0]
// "1.4.0"      → parts [1,4,0]  → equal for compatibility
// "dev"        → parts [0,0,0]
```

Install blocked when `compareSemver(minGoSiteVersion, hostVersion) > 0`.
