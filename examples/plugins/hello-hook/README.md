# hello-hook reference plugin

A minimal Tier 1 plugin that demonstrates the host hook bus contract.
Implements every method of `pluginrpc.Plugin` and declares
`nginx.before_reload`, `nginx.after_reload`, and `site.before_create` in
its manifest. The handlers are no-ops — the value of this plugin is
showing how to wire the build, sign, install, and observe end-to-end.

## Build

```sh
cd examples/plugins/hello-hook
make build
```

Produces `dist/gosite-hello-hook.zip` containing the manifest, the
validate binary, and the runtime binary. Sign the zip and install via:

```sh
curl -X POST -F artifact=@dist/gosite-hello-hook.zip \
  -F sha256=$(sha256sum dist/gosite-hello-hook.zip | awk '{print $1}') \
  http://localhost:8080/api/v1/plugins/install
```

## Why no `.proto`?

The contract (`pkg/pluginrpc`) is exposed as a Go interface that doubles
as a HashiCorp net/rpc surface. The same struct types are gRPC-ready,
so adopting protobuf codegen is a non-breaking evolution — the
`PluginServerFactory` map and the gRPC plugin descriptor can be
generated side-by-side.
