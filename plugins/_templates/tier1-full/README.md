# Tier 1 — full production template

Demonstrates every supported Tier 1 capability from
[`19-plugin-installer.md`](../../docs/sequences/19-plugin-installer.md):

| Area | Where |
|------|-------|
| All lifecycle hooks | `manifest.json` `capabilities.hooks` |
| Logging sink | `logging.on_event` + `capabilities.loggingSink` |
| Config + UI | `config/schema.json`, `ui.sidebar` |
| Config migration | `MigrateConfig` in `main.go` (v1 → v2) |
| Validate contract | `Validate` checks tier + rpcVersion |

## Config migration example

When switching to a version with `configVersion: "2"`, the host calls
`MigrateConfig`. This template renames `webhookUrl` → `endpoint`.

## Build

```bash
make build
make sign KEY=~/.config/gosite/signing.key KEY_ID=template-1
make install
```

Host stores config via
`PUT /api/v1/plugins/template/full/versions/0.1.0/config`.
Secret fields (`apiToken`) are encrypted at rest.
