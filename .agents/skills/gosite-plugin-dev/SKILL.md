---
name: gosite-plugin-dev
description: Develop, build, sign, install, and publish GoSite control-plane plugins (tier 0 webhooks, tier 1 go-plugin). Use when working in plugins/, manifest.json, gosite.plugin.json, plugin hooks, artifact signing, remote install from GitHub/GitLab, or sequences 19/20 plugin installer docs.
---

# GoSite Plugin Development

Develop extensions for GoSite's privileged control plane (nginx, Docker, SSL, jobs). Plugins are **not** arbitrary JS — tier 1 is a go-plugin subprocess; tier 0 is manifest + HTTP webhooks.

## Before coding

1. Read repo docs (in order):
   - `docs/sequences/19-plugin-installer.md` — lifecycle, hooks, signing
   - `docs/sequences/20-plugin-remote-distribution.md` — remote install, `gosite.plugin.json`
   - `plugins/_templates/README.md` — templates and artifact layout
2. Pick tier (see decision tree below).
3. Copy the matching template to `plugins/<vendor>/<name>/` (never edit `_templates/` in place for production plugins).

## Tier decision

| Need | Tier | Template |
|------|------|----------|
| Forward events to external HTTP service (Slack, SaaS) | 0 | `plugins/_templates/tier0-webhook/` |
| Custom logic in host subprocess (hooks, config, UI schema) | 1 | `tier1-minimal/` (start) or `tier1-full/` (production) |
| Sandboxed community validators | 2 | **Deferred** — stub only |
| `.so` native plugin | 3 | **Vendor-only** — not for community |

**Rule:** declare every hook in `capabilities.hooks` (tier 1) or `webhooks[].event` (tier 0). Undeclared hooks are not dispatched; runtime must not assume undeclared host access.

## Scaffold workflow

```bash
# From gosite repo root
cp -r plugins/_templates/tier1-minimal plugins/acme/my-plugin
cd plugins/acme/my-plugin
```

Edit `manifest.json`:

- `id`: `vendor/name` (globally unique, lowercase)
- `apiVersion`: `gosite-plugin/1`
- `rpcVersion`: `1` (tier 1 only)
- `minGoSiteVersion`: set honestly
- `capabilities.hooks`: only events you handle

Update parent `Makefile` variables: `PLUGIN_ID`, `PLUGIN_NAME`, `MAIN_PKG`.

Implement `pkg/pluginrpc` methods in `main.go`:

- `Validate` — install-time dry run; return errors to block install
- `Health` — liveness for supervisor
- `CallHook` — handle declared events; respect `req.Strict` (blocking on `*.before_*`)
- `MigrateConfig` — required when `configVersion` changes across switch

Use shared serve helper:

```go
import "github.com/jahrulnr/gosite/plugins/_templates/_shared/rpcplugin"
// rpcplugin.Serve(myPlugin{})
```

## Build, sign, install (local)

```bash
make build          # dist/<name>.zip + sha256
make vet            # go vet
make sign KEY=~/.config/gosite/signing.key KEY_ID=acme-1   # production
make install GOSITE_URL=http://127.0.0.1:8080 AUTH_USER=admin AUTH_PASS=...
```

**Artifact zip layout (tier 1):**

```text
manifest.json
plugin/gosite      # runtime binary
plugin/validate    # install probe (often copy of gosite)
```

Tier 0 zip may contain **only** `manifest.json`.

**Dev unsigned:** `PLUGIN_ALLOW_UNSIGNED=true` on host. Signing embeds sigs into zip and changes bytes — use `uploadDigest` from `.sigmeta` when uploading with expected sha256.

## Lifecycle API (after install)

| Action | Endpoint |
|--------|----------|
| Install (upload) | `POST /api/v1/plugins/install` multipart `artifact` + optional `sha256` |
| Install (remote) | `POST /api/v1/plugins/install` JSON `{ "source": {...}, "permissions_ack": true }` |
| Resolve preview | `POST /api/v1/plugins/install/resolve` |
| Enable | `POST /api/v1/plugins/{vendor}/{name}/enable` `{ "version": "..." }` |
| Disable | `POST /api/v1/plugins/{vendor}/{name}/disable` |
| Switch version | `POST /api/v1/plugins/{vendor}/{name}/switch` `{ "version": "..." }` |
| Config | `PUT /api/v1/plugins/{vendor}/{name}/versions/{v}/config` |
| Uninstall | `DELETE /api/v1/plugins/{vendor}/{name}/versions/{v}` |

**States:** `installing → installed | install_failed`; `installed → enabling → enabled | enable_failed`; `enabled → disabling → installed`.

Install ≠ enable. Remote install never auto-activates. Enable blocked until `permissions_ack` for remote installs.

## Remote publish (Path A — release zip)

Add `gosite.plugin.json` at repo root (distribution index). Tag + CI uploads signed zip per `GOOS/ARCH`. Host resolves asset by platform + pinned `sha256` — **not** filename glob.

See [examples.md](examples.md) for `gosite.plugin.json` and `source` JSON shapes.

**Path B (community):** tag only + `distribution.build` block; host builds via Docker (`PLUGIN_BUILD_ENABLED`). Go-only in wave G.

**CLI:**

```bash
gosite plugin resolve --source '{"type":"github-release","repo":"acme/pkg","tag":"v1.0.0"}'
gosite plugin install --source '...' --permissions-ack
```

## UI contributions

Plugins ship **data only** — no custom JS/HTML.

```json
"ui": {
  "sidebar": [{ "label": "Settings", "route": "/plugins/vendor/name/settings" }]
}
```

Routes must start with `/plugins/<id>/`. Host renders sidebar + JSON-schema config forms. Secret config fields are write-only in API responses.

## Hook semantics (implementer checklist)

- `*.before_*` events are **strict** by default — hard error/timeout blocks the host operation
- Order: `plugin.id` asc, then `version` desc
- Timeout: `PLUGIN_HOOK_TIMEOUT` (default 5s)
- `hookIsolation: independent` allows concurrent dispatch only on lenient events
- Tier 0: host POSTs to `webhooks[].url` with `X-Gosite-Webhook-Event` + `X-Gosite-Webhook-Secret`

Full hook map: [reference.md](reference.md) or `plugins/_templates/docs/HOOKS.md`.

## Config migration (switch)

When bumping `configVersion`:

1. Implement `MigrateConfig` in tier 1 plugin
2. Host calls it during `switch` before starting new runtime
3. Migration failure → switch rejected; previous version stays enabled

## Validation before PR / ship

```bash
# From gosite repo root
go test ./internal/service/plugin/...
go run plugins/_templates/_shared/scripts/verify.go -artifact plugins/acme/my-plugin/dist/*.zip -pub signing.pub.json
cd web && npm test   # if panel/API client touched
```

Manual smoke: install → enable → trigger one declared hook → disable → uninstall.

## Touch map (host code)

Changing plugin contracts may require edits in:

| Area | Path |
|------|------|
| RPC contract | `pkg/pluginrpc/` |
| Install/lifecycle | `internal/service/plugin/service.go` |
| Remote resolver | `internal/service/plugin/remote/` |
| HTTP API | `internal/delivery/http/handler/plugin.go` |
| Panel | `web/src/views/Plugins*.tsx` |
| Templates/docs | `plugins/_templates/` |

## Anti-patterns

- Shipping arbitrary frontend assets or routes outside `/plugins/<id>/`
- Assuming hooks run without listing them in manifest
- Blocking lenient hooks with errors (only strict events should hard-fail)
- Using tier 3 `.so` for community plugins
- Relying on `gosite*.zip` filename heuristics for GitHub releases — use `gosite.plugin.json` assets table

## Additional resources

- Manifest field reference: `plugins/_templates/docs/MANIFEST.md`
- Signing: `plugins/_templates/docs/SIGNING.md`
- Lifecycle (developer): `plugins/_templates/docs/LIFECYCLE.md`
- Architecture tiers: `docs/architecture/plugin-platform.md`
- API/failure classes/env vars: [reference.md](reference.md)
- JSON examples: [examples.md](examples.md)
