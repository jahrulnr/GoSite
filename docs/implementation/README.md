# GoSite Implementation Waves

Subagent-oriented implementation plan for the GoSite oneshot backend.

## Waves

| Wave | Scope | Gate |
|------|-------|------|
| [WAVE-SA-1](./WAVE-SA-1.md) | Foundation: go.mod, config, migrations, sqlite repo, testutil | Named tests green |
| [WAVE-SA-2](./WAVE-SA-2.md) | Runtime: bootstrap, Dockerfile, compose, templates | `gosite init` idempotent |
| [WAVE-SA-3](./WAVE-SA-3.md) | Auth: basic auth + session login | 8+ auth tests |
| [WAVE-SA-4](./WAVE-SA-4.md) | Website + nginx + SSL | 25+ tests, legacy bug fixes |
| [WAVE-SA-5](./WAVE-SA-5.md) | Docker, files, mount, cron, jobs | 10 tests per package |
| [WAVE-SA-6](./WAVE-SA-6.md) | Splunk Lite + Grafana Lite + audit | Query/metrics tests |
| [WAVE-SA-7](./WAVE-SA-7.md) | System, settings, logs, database viewer | Dashboard aggregate |
| [WAVE-PLUGIN-G](./WAVE-PLUGIN-G.md) | Remote plugin distribution (seq 20) | Shipped v1.3.1 |

## Shared contracts (Wave 0)

- `pkg/apperror/` — structured error codes
- `internal/contracts/` — `NginxRunner`, `CommandRunner`, `DockerClient`, `AuditWriter`
- `internal/testutil/` — fixtures and mocks

## Verification gate (every wave)

1. `go test -race -count=1` on wave packages exits 0
2. No `TODO` stubs in delivered endpoints
3. Named tests from wave brief exist and assert real behavior
4. Parent runs full `make test` before merge

## References

- [architecture/overview.md](../architecture/overview.md)
- [architecture/plugin-platform.md](../architecture/plugin-platform.md)
- [reference/api-inventory.md](../reference/api-inventory.md)
- [sequences/](../sequences/)
- [DOCS-MAINTENANCE.md](../DOCS-MAINTENANCE.md)
