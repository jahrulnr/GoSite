# Tier 0 — HTTP webhook plugin

Manifest-only artifact. GoSite POSTs lifecycle events to the URLs in
`webhooks[]` — no go-plugin subprocess on the host.

## Flow

1. Run a receiver (this template includes a dev server on `:9191`).
2. Point `webhooks[].url` in `manifest.json` at your endpoint.
3. `make build` → `dist/gosite-template-webhook.zip` (manifest only).
4. Install + enable through the panel or `make install`.

## Webhook contract

Headers:

- `Content-Type: application/json`
- `X-Gosite-Webhook-Event` — event name (e.g. `nginx.before_reload`)
- `X-Gosite-Webhook-Secret` — shared secret (`PLUGIN_WEBHOOK_SECRET` on host)

Body:

```json
{
  "event": "nginx.before_reload",
  "plugin": "http://127.0.0.1:9191/gosite",
  "payload": { ... }
}
```

Return HTTP 2xx for success. 5xx responses fail strict hooks.

## Local test

```bash
# terminal 1
make receiver

# terminal 2
make build
make install GOSITE_URL=http://127.0.0.1:8080
# enable plugin in panel, then trigger nginx reload
```

See [`../docs/HOOKS.md`](../docs/HOOKS.md) for the full event map.
