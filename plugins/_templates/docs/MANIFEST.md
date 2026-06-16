# Manifest reference

Minimum install contract (`manifest.json` inside the artifact zip).

## Required fields

```json
{
  "id": "vendor/name",
  "name": "Human Name",
  "version": "1.0.0",
  "tier": 1,
  "apiVersion": "gosite-plugin/1",
  "minGoSiteVersion": "0.0.0",
  "rpcVersion": "1"
}
```

| Field | Notes |
|-------|-------|
| `id` | Globally unique, pattern `vendor/name` (lowercase) |
| `tier` | `0` (webhook) or `1` (go-plugin). Tiers 2–3 not supported yet. |
| `apiVersion` | Must be `gosite-plugin/1` |
| `rpcVersion` | Required for tier 1; must be `1` |
| `configVersion` | Optional schema version; used on switch / `MigrateConfig` |

## Capabilities (tier 1)

```json
"capabilities": {
  "hooks": ["nginx.before_reload"],
  "hookIsolation": "sequential",
  "uiSidebar": true,
  "configSchema": true,
  "loggingSink": true,
  "rulesAndRoles": "declarative"
}
```

## Tier 0 webhooks

```json
"webhooks": [
  {
    "event": "nginx.before_reload",
    "url": "https://hooks.example.com/gosite",
    "method": "POST"
  }
]
```

Body shape:

```json
{
  "event": "nginx.before_reload",
  "plugin": "https://hooks.example.com/gosite",
  "payload": { ... }
}
```

## Entrypoints (tier 1)

```json
"entrypoints": {
  "validate": { "type": "go-plugin", "command": "plugin/validate" },
  "runtime":  { "type": "go-plugin", "command": "plugin/gosite" }
}
```

`validate` runs at install time (subprocess, deadline). `runtime` starts on
enable.

## UI contributions (data-only)

Plugins never ship JS. Host renders sidebar + config forms:

```json
"ui": {
  "sidebar": [
    { "label": "Settings", "route": "/plugins/vendor/name/settings" }
  ]
}
```

Routes **must** start with `/plugins/<id>/`.

Config values are stored via `PUT /api/v1/plugins/{vendor}/{name}/versions/{version}/config`.
Secret fields are encrypted at rest (`PLUGIN_CONFIG_KEY`).

## Artifact integrity

```json
"artifact": { "sha256": "<lowercase hex of zip bytes>" },
"signatures": [{ "keyId": "vendor-1", "sig": "<base64 ed25519>" }]
```

See [`SIGNING.md`](SIGNING.md).
