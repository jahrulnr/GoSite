# Wave B — Built-in plugins

Subagent-oriented tracker for [sequence 23](../sequences/23-builtin-plugins.md).

**Status:** Not started

**Detailed checklist:** [23-builtin-plugins-impl.md](../sequences/23-builtin-plugins-impl.md)

## Scope

| Phase | Status | Doc |
|-------|--------|-----|
| **B1** Infrastructure | Not started | SeedBundled, bundled index, Dockerfile |
| **B2** `gosite/mcp` + UI | Not started | First built-in, panel badges |
| **B3** Upgrade + restore | Not started | Digest bump, restore API |
| **B4** Deferred | — | Second official plugin, catalog flag |

## Problem summary

Plugin platform (seq 19–21) requires manual install. Official `gosite/mcp` does not appear in the panel until an operator uploads or fetches from GitHub. **Path C:** ship plugin zips inside the GoSite image, seed registry as `installed` / disabled by default.

## Gate

B2 minimum ship when B1-1–B1-7 and B2-1–B2-6 in [23-builtin-plugins-impl.md](../sequences/23-builtin-plugins-impl.md) are checked and:

1. `go test -race -count=1` on `internal/service/plugin/bundled` and seed tests exits 0
2. Fresh init → `gosite/mcp` visible in plugin registry (`installed`, not `enabled`)
3. Enable → seq 21 Integration tab + token API smoke passes
4. [mcp-operator.md](../guides/mcp-operator.md) reflects auto-install step

## References

- [DOCS-MAINTENANCE.md](../DOCS-MAINTENANCE.md)
- [plugin-platform.md](../architecture/plugin-platform.md) — phase P7
- [21-plugin-mcp.md](../sequences/21-plugin-mcp.md)
- [plugins/gosite/mcp/](../../plugins/gosite/mcp/)
