# Tier 2 — WebAssembly (deferred)

**Not installable today.** GoSite currently accepts only tier `0` and `1`
plugins (`plugin.Service` returns `only tier 0 and tier 1 plugins are
supported`).

This folder documents the intended future path for sandboxed community
validators/transformers (Extism-style host).

## Planned artifact layout

```text
manifest.json
plugin/module.wasm      # WASM module with stable host imports
plugin/validate.wasm    # optional smaller validate entry
```

## Planned contract

- Same event names as Tier 1 (`capabilities.hooks`)
- Host provides memory-safe imports: config read, structured logging, limited HTTP egress
- `hookIsolation: independent` for lenient events only
- No arbitrary filesystem or subprocess access

## Why WASM later

| Concern | Tier 1 go-plugin | Tier 2 WASM |
|---------|------------------|-------------|
| Isolation | subprocess | sandboxed VM |
| Language | Go (shared module) | Rust, TinyGo, AssemblyScript |
| Community | vendor builds binary | safer third-party modules |

## Reference manifest

See [`manifest.json`](manifest.json) — **do not** `make install`; use for
design reviews only.

When Tier 2 lands, this template will gain a `Makefile`, sample module, and
host import documentation.
