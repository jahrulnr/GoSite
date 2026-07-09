# Wave P6 — MCP integration & integration tokens

Subagent-oriented tracker for [sequence 21](../sequences/21-plugin-mcp.md).

**Status:** P6-host-auth + P6b implemented (P6a external repo pending)

**Detailed checklist:** [21-plugin-mcp-impl.md](../sequences/21-plugin-mcp-impl.md)

## Scope

| Phase | Status | Doc |
|-------|--------|-----|
| **P6-host-auth** | Not started | [integration-tokens.md](../reference/integration-tokens.md), [plugin-integration-auth.md](../architecture/plugin-integration-auth.md) |
| **P6a** | Not started | [mcp-operator.md](../guides/mcp-operator.md) |
| **P6b** | Not started | [mcp-tools.md](../reference/mcp-tools.md) |
| **P6c** | Blocked | Third listener architecture sequence required |

## Gate

P6-host-auth complete when H1–H5 in [21-plugin-mcp-impl.md](../sequences/21-plugin-mcp-impl.md) are checked and:

1. `go test -race -count=1` on new packages exits 0
2. OpenAPI documents integration token paths
3. Splunk-lite can query `integration_token.*` audit events

## References

- [DOCS-MAINTENANCE.md](../DOCS-MAINTENANCE.md)
- [plugin-platform.md](../architecture/plugin-platform.md) — P6 phase
- [plugin-permissions.md](../reference/plugin-permissions.md)
