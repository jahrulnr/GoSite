# GoSite Plugin — Reference

## Host hook events

| Event | Strict default | Payload summary |
|-------|----------------|-----------------|
| `site.before_create` | yes | `domain`, `path`, `type`, `upstream`, `active` |
| `site.after_enable` | lenient | `id`, `domain` |
| `site.config_changed` | yes | `id`, `domain`, `type`, optional `raw` |
| `ssl.before_issue` | yes | `website_id`, `domain` |
| `ssl.after_renew` | lenient | `website_id`, `domain`, `manual` |
| `nginx.before_reload` | yes | `global_config`, `site_dir`, `active_dir` |
| `nginx.after_reload` | lenient | same as before |
| `job.before_run` | yes | `id`, `job_type`, `name` |
| `job.after_run` | lenient | `id`, `job_type`, `name` |
| `job.on_failure` | lenient | `id`, `job_type`, `name`, `error` |
| `cron.before_trigger` | yes | `id`, `name`, `run_every` |
| `container.before_action` | yes | `id`, `action` (`restart` / `stop`) |

## manifest.json required fields

```json
{
  "id": "vendor/name",
  "name": "Human Name",
  "version": "1.0.0",
  "tier": 1,
  "apiVersion": "gosite-plugin/1",
  "minGoSiteVersion": "1.3.0",
  "rpcVersion": "1"
}
```

Optional: `configVersion`, `capabilities`, `permissions`, `entrypoints`, `ui`, `webhooks`, `artifact`, `signatures`.

## Lifecycle failure classes (install/enable)

| Class | Meaning |
|-------|---------|
| `validate_timeout` | Validate subprocess exceeded deadline |
| `start_failed` | Runtime failed to start on enable |
| `hook_refresh_failed` | Dispatcher refresh failed after start |
| `stop_failed` | Stop runtime failed on disable/switch |
| `fs_delete_failed` | Artifact delete failed on uninstall |
| `compensation_failed` | Manual intervention required before retry |

## Remote install failure classes

| Class | Meaning |
|-------|---------|
| `resolve_failed` | Bad ref, unknown repo, no matching asset |
| `fetch_failed` | Network / 404 / timeout |
| `fetch_digest_mismatch` | Downloaded bytes ≠ pinned sha256 |
| `release_integrity_failed` | Index digest ≠ actual release asset |
| `platform_unsupported` | No asset for host GOOS/ARCH |
| `auth_token_expired` | GitHub/GitLab token rejected |
| `resolve_stale` | Preview digest/commit changed (TOCTOU) |
| `operation_in_progress` | Concurrent lifecycle op on same plugin_id |

## Source types (`source.type`)

| Type | Notes |
|------|-------|
| `url` | HTTPS + pinned sha256 + allowlist |
| `github-release` | `gosite.plugin.json` at tag, prefer-release |
| `gitlab-release` | Same index contract |
| `github-build` / `gitlab-build` | Docker builder Path B |
| `catalog` | Bundled curated index |
| `git-ref` | Tier 0 manifest at tag only |

`installPath`: `auto` | `release` | `build` (default `auto`).

## Key environment variables

| Variable | Purpose |
|----------|---------|
| `PLUGIN_ALLOW_UNSIGNED` | Dev: skip signature check |
| `PLUGIN_TRUST_MODE` | `strict` \| `community` \| `dev` |
| `PLUGIN_REMOTE_INSTALL` | Enable remote install sources |
| `PLUGIN_INSTALL_ALLOWED_HOSTS` | Fetch allowlist |
| `PLUGIN_FETCH_MAX_BYTES` | Default 64MiB |
| `PLUGIN_FETCH_TIMEOUT` | Default 120s |
| `PLUGIN_HOOK_TIMEOUT` | Per-hook deadline |
| `PLUGIN_CONFIG_KEY` | AES key for secret config fields |
| `PLUGIN_WEBHOOK_SECRET` | Tier 0 webhook header secret |
| `GITHUB_TOKEN` / `GITLAB_TOKEN` | Private repo fetch |
| `PLUGIN_BUILD_ENABLED` | Allow Path B Docker build |
| `PLUGIN_BUILD_TIMEOUT` | Default 600s |

## Trust modes

| Mode | Behaviour |
|------|-----------|
| `strict` | Digest + signature required (Path A); production default |
| `community` | Unsigned shows warning; Path B allowed |
| `dev` | `PLUGIN_ALLOW_UNSIGNED=true` |

## Keyring

```http
POST /api/v1/plugins/keyring
{ "vendor": "acme", "keyId": "acme-1", "publicKey": "<base64 ed25519 pubkey>" }
```

Signature is Ed25519 over lowercase hex SHA-256 of **uploaded** zip bytes.

## Provenance columns (registry)

After remote install: `source_type`, `source_ref`, `resolved_url`, `resolved_digest`, `artifact_digest`, `source_commit`, `install_path`, `permissions_ack_at`, `permissions_acked_caps`.

## Deferred / out of scope

- Tier 2 WASM, tier 3 community `.so`
- Scoped plugin API tokens + runtime egress policy (partial)
- Semver range resolution, transitive plugin deps
- SLSA attestation (L3)
