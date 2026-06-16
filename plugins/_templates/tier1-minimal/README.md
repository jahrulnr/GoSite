# Tier 1 — minimal go-plugin

Smallest HashiCorp go-plugin binary with one hook. Copy this folder to
`plugins/<vendor>/<name>/` and change `manifest.json` + `PLUGIN_ID` in the
Makefile.

## Build

```bash
make build    # dist/gosite-template-minimal.zip
make vet
make sign KEY=~/.config/gosite/signing.key KEY_ID=template-1
make install GOSITE_URL=http://127.0.0.1:8080
```

Implements [`pkg/pluginrpc`](../../pkg/pluginrpc/contract.go) via
[`_shared/rpcplugin`](../_shared/rpcplugin/serve.go).

Working reference: [`examples/plugins/hello-hook`](../../examples/plugins/hello-hook).
