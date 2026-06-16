# Host hook map

Hooks fire from GoSite service-layer lifecycle points **before** irreversible
side effects (nginx reload, job run, SSL issue, etc.).

Source: [`docs/architecture/plugin-platform.md`](../../docs/architecture/plugin-platform.md).

## Events

| Event | Package | Strict default | Payload (summary) |
|-------|---------|----------------|-------------------|
| `site.before_create` | `service/website` | yes (`*.before_*`) | `domain`, `path`, `type`, `upstream`, `active` |
| `site.after_enable` | `service/website` | lenient | `id`, `domain` |
| `site.config_changed` | `service/website` | yes | `id`, `domain`, `type`, optional `raw` |
| `ssl.before_issue` | `service/ssl` | yes | `website_id`, `domain` |
| `ssl.after_renew` | `service/ssl` | lenient | `website_id`, `domain`, `manual` |
| `nginx.before_reload` | `infra/nginx` | yes | `global_config`, `site_dir`, `active_dir` |
| `nginx.after_reload` | `infra/nginx` | lenient | same as before |
| `job.before_run` | `infra/job` | yes | `id`, `job_type`, `name` |
| `job.after_run` | `infra/job` | lenient | `id`, `job_type`, `name` |
| `job.on_failure` | `infra/job` | lenient | `id`, `job_type`, `name`, `error` |
| `cron.before_trigger` | `service/cron` | yes | `id`, `name`, `run_every` |
| `container.before_action` | `service/docker` | yes | `id`, `action` (`restart` / `stop`) |

## Dispatch rules

Declare hooks in manifest `capabilities.hooks` (tier 1) or `webhooks[].event`
(tier 0).

| Concept | Default | Override |
|---------|---------|----------|
| Order | `plugin.id` asc, then `version` desc | — |
| Timeout | 5s (`PLUGIN_HOOK_TIMEOUT`) | env |
| Strict | `*.before_*` events block caller on error | `StrictEvents` map |
| Isolation | `sequential` | manifest `hookIsolation: independent` (lenient only) |
| Circuit breaker | 3 failures / 5m window | host config |

## Tier 0 vs tier 1

- **Tier 1** — host calls your go-plugin `CallHook` RPC.
- **Tier 0** — host POSTs JSON to `webhooks[].url` with headers
  `X-Gosite-Webhook-Event` and `X-Gosite-Webhook-Secret` (`PLUGIN_WEBHOOK_SECRET`).

Use [`tier0-webhook/dev-receiver`](../tier0-webhook/dev-receiver/) to test
webhook payloads locally.
