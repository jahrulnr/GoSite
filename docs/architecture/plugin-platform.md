# ADR: Plugin platform

**Status:** Implemented (tier 0 + tier 1; tier 2 WASM deferred)
**Date:** 2026-06-16  
**Research:** dev-docs `go-plugin-platform` — see [plugin architecture](https://github.com/jahrulnr/dev-docs/blob/main/docs/best-practices/architecture/patterns/plugin-architecture_en.md) (or local dev-docs corpus)

## Context

GoSite is a monolithic hosting control plane (`handler → service → infra`) with privileged access to nginx, docker, filesystem, and jobs. The product vision includes **extensibility**: custom deploy flows, integrations (including AI via 9router), and eventually a plugin catalog—not a single “AI feature” bolt-on.

Training knowledge and KrakenD-style `.so` plugins are **not** sufficient guidance: KrakenD Community Edition removes Go plugin support in v3.0 because OSS build pipelines cannot be supported symmetrically.

## Decision

Adopt a **tiered plugin model**:

| Tier | Mechanism | GoSite use |
| --- | --- | --- |
| **0** | HTTP webhooks + scoped API tokens | Notifications, DNS, AI router, SaaS |
| **1** | HashiCorp **go-plugin** (gRPC subprocess) | Deploy providers, nginx snippets, health probes |
| **2** | **WebAssembly** (Extism-style host) | Community validators/transformers (later) |
| **3** | Go stdlib `plugin` `.so` | **Not** for community; vendor-curated only if ever |

Implement an internal **hook dispatcher** at service-layer lifecycle points before irreversible side effects (nginx reload, job run, SSL issue).

## Hook map (initial)

| Area | Package | Proposed hooks |
| --- | --- | --- |
| Websites | `service/website` | `site.before_create`, `site.after_enable`, `site.config_changed` |
| SSL | `service/ssl` | `ssl.before_issue`, `ssl.after_renew` |
| nginx | `infra/nginx` | `nginx.before_reload`, `nginx.after_reload` |
| Jobs | `infra/job` | `job.before_run`, `job.after_run`, `job.on_failure` |
| Cron | `service/cron` | `cron.before_trigger` |
| Docker | `service/docker` | `container.before_action` |

Rollback on failed nginx reload (existing pattern in `website.Toggle`) applies to plugin-induced reloads.

## Consequences

**Positive**

- Aligns with Terraform/Vault operational model (go-plugin).
- Crash and upgrade isolation without Go ABI lock of `.so` plugins.
- Clear path to marketplace: Tier 0 + signed Tier 1 binaries first.

**Negative**

- RPC interface design and versioning work before first plugin.
- Subprocess plugins add latency vs in-process (acceptable for control-plane ops).
- WASM tier requires careful host-function design.

## Phased delivery

1. **P0** — Hook bus core is implemented for enabled plugin dispatch, strict/lenient behavior, per-hook timeout, circuit breaker, audit error writes, and initial call sites across nginx, website, SSL, jobs, cron, and Docker. Tier 0 webhook transport is still pending.
2. **P1** — Plugin registry (SQLite), manifest, checksum/signature verify, lifecycle API, purge, reconcile, and admin UI are implemented.
3. **P2** — go-plugin SDK + reference plugin are pending.
4. **P3** — Host-rendered plugin admin UI exists; richer catalog/config UI is pending.
5. **P4+** — WASM sandbox, marketplace publishing, encrypted config storage, and full network isolation remain future security-review work.

## Alternatives considered

| Alternative | Rejected because |
| --- | --- |
| KrakenD-style `.so` marketplace | CE deprecation; ABI/support burden on OSS users |
| Fork-only customization | Poor upgrade/security path for users |
| Lua/Yaegi in core | Useful for middleware-style hooks only; unsafe for untrusted authors |
| Template-only (Coolify model) | Valuable for catalog but insufficient for arbitrary deploy logic |

## References

- HashiCorp go-plugin: https://github.com/hashicorp/go-plugin
- KrakenD dropping CE plugins: https://www.krakend.io/blog/dropping-plugins-support-on-community/
- Go `plugin` package warnings: https://pkg.go.dev/plugin
- Traefik plugin manifests (yaegi/wasm): https://github.com/traefik/traefik/tree/master/pkg/plugins
- Extism: https://github.com/extism/extism
