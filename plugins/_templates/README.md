# GoSite plugin templates

Prototypes for every supported plugin development path described in
[`docs/sequences/19-plugin-installer.md`](../../docs/sequences/19-plugin-installer.md).

Pick a template folder, copy it to your vendor namespace (e.g.
`plugins/acme/my-plugin/`), adjust `manifest.json`, then build the zip
artifact and install through the panel or API.

## Which template?

| Path | Tier | When to use |
|------|------|-------------|
| [`tier0-webhook/`](tier0-webhook/) | 0 | External HTTP receiver (SaaS, Slack, 9router). Manifest + `webhooks[]` only — no subprocess logic on the host. |
| [`tier1-minimal/`](tier1-minimal/) | 1 | Smallest go-plugin binary; one or two hooks; good first plugin. |
| [`tier1-full/`](tier1-full/) | 1 | Production-shaped: all hook types, config + UI contributions, config migration, logging sink. |
| [`tier2-wasm/`](tier2-wasm/) | 2 | **Deferred** — manifest stub + design notes for future WASM sandbox. |
| [`tier3-native-so/`](tier3-native-so/) | 3 | **Not for community** — vendor-only `.so` notes. |

Shared tooling lives under [`_shared/`](_shared/).

## Quick start (tier 1)

```bash
cd plugins/_templates/tier1-minimal
cp -r . ../../acme/my-first-plugin    # or work in-place for experiments
make build
make sign KEY=~/.config/gosite/signing.key   # optional in dev if PLUGIN_ALLOW_UNSIGNED=true
make install GOSITE_URL=https://your-host:1100
```

## Artifact layout (zip)

Every installable artifact is a zip file:

```text
manifest.json          # required — immutable contract snapshot
plugin/gosite          # tier 1 runtime binary (go-plugin serve)
plugin/validate        # tier 1 validate binary (may be a copy of gosite)
```

Tier 0 artifacts may contain **only** `manifest.json` (no `plugin/` tree).

## Documentation

- [`docs/HOOKS.md`](docs/HOOKS.md) — host hook map and strict/lenient semantics
- [`docs/MANIFEST.md`](docs/MANIFEST.md) — manifest field reference
- [`docs/LIFECYCLE.md`](docs/LIFECYCLE.md) — install → enable → switch → uninstall
- [`docs/SIGNING.md`](docs/SIGNING.md) — Ed25519 artifact signing

## Contract

Tier 1 plugins implement [`pkg/pluginrpc`](../../pkg/pluginrpc/contract.go):

- `Validate` — install-time dry run
- `Health` — supervisor liveness probe
- `CallHook` — lifecycle event handler
- `MigrateConfig` — version switch config migration

See also the working reference: [`tier1-minimal/`](tier1-minimal/).
