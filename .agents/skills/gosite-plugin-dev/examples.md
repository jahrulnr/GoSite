# GoSite Plugin — Examples

## Tier 1 minimal manifest

```json
{
  "id": "acme/hello",
  "name": "Acme Hello",
  "version": "1.0.0",
  "tier": 1,
  "apiVersion": "gosite-plugin/1",
  "minGoSiteVersion": "1.3.0",
  "rpcVersion": "1",
  "configVersion": "1",
  "capabilities": {
    "hooks": ["nginx.before_reload"],
    "hookIsolation": "sequential"
  },
  "entrypoints": {
    "validate": { "type": "go-plugin", "command": "plugin/validate" },
    "runtime": { "type": "go-plugin", "command": "plugin/gosite" }
  }
}
```

## Tier 0 webhook manifest

```json
{
  "id": "acme/slack-forwarder",
  "name": "Slack Forwarder",
  "version": "1.0.0",
  "tier": 0,
  "apiVersion": "gosite-plugin/1",
  "minGoSiteVersion": "1.3.0",
  "capabilities": {
    "hooks": ["job.on_failure", "nginx.before_reload"]
  },
  "webhooks": [
    {
      "event": "job.on_failure",
      "url": "https://hooks.example.com/gosite",
      "method": "POST"
    }
  ]
}
```

## Tier 1 CallHook (Go)

```go
func (p *plugin) CallHook(req pluginrpc.CallHookRequest, resp *pluginrpc.CallHookResponse) error {
    switch req.EventName {
    case "nginx.before_reload":
        // parse req.PayloadJSON; return error only if blocking is intended
        resp.Status = "ok"
        return nil
    default:
        resp.Status = "ok"
        return nil
    }
}
```

## gosite.plugin.json (Path A — release)

```json
{
  "id": "acme/hello",
  "name": "Acme Hello",
  "repository": "https://github.com/acme/gosite-plugin-hello",
  "distribution": {
    "apiVersion": "gosite-plugin-distribution/1",
    "build": {
      "language": "go",
      "entrypoint": "./cmd/plugin",
      "goVersion": "1.22"
    },
    "releases": [
      {
        "version": "1.0.0",
        "minGoSiteVersion": "1.3.0",
        "sourceCommit": "abc123def456",
        "sourceRepository": "https://github.com/acme/gosite-plugin-hello",
        "assets": [
          {
            "name": "gosite-hello.zip",
            "os": "linux",
            "arch": "amd64",
            "url": "https://github.com/acme/gosite-plugin-hello/releases/download/v1.0.0/gosite-hello-linux-amd64.zip",
            "sha256": "…",
            "signatures": [{ "keyId": "acme-1", "sig": "…" }]
          }
        ]
      }
    ]
  }
}
```

## Remote install source JSON

**GitHub release:**

```json
{
  "source": {
    "type": "github-release",
    "repo": "acme/gosite-plugin-hello",
    "tag": "v1.0.0",
    "installPath": "auto"
  },
  "permissions_ack": true
}
```

**Direct URL (G1):**

```json
{
  "source": {
    "type": "url",
    "url": "https://cdn.example.com/gosite-hello-1.0.0.zip",
    "sha256": "abc123…"
  },
  "permissions_ack": true
}
```

**Catalog:**

```json
{
  "source": {
    "type": "catalog",
    "pluginId": "acme/hello",
    "version": "1.0.0"
  },
  "permissions_ack": true
}
```

## Makefile (tier 1)

```makefile
PLUGIN_ID   := acme/hello
PLUGIN_NAME := gosite-hello
MAIN_PKG    := ./plugins/acme/hello

include ../../_templates/_shared/Makefile.inc

.PHONY: build vet
build: _build-bin _pack-zip
vet:
	cd $(GOSITE_ROOT) && go vet $(MAIN_PKG)
```

## Local dev loop

```bash
cd plugins/acme/hello
make build
make install GOSITE_URL=http://127.0.0.1:8080

curl -u admin:pass -X POST \
  http://127.0.0.1:8080/api/v1/plugins/acme/hello/enable \
  -H 'Content-Type: application/json' \
  -d '{"version":"1.0.0"}'
```

## Tier 0 local webhook receiver

```bash
cd plugins/_templates/tier0-webhook/dev-receiver
go run .   # listens :9191
# Point manifest webhooks[] at http://127.0.0.1:9191/gosite
```
