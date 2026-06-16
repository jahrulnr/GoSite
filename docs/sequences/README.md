# Sequence Diagrams — Index

GoSite feature flow diagrams. **Legacy BangunSite** sections (if any) are kept as migration reference.

## Runtime & infrastructure

| # | File | Feature | Status |
|---|------|-------|--------|
| 01 | [01-container-startup.md](./01-container-startup.md) | `start.sh`, `gosite init`, nginx-repair | ✅ |
| 02 | [02-tls-proxy.md](./02-tls-proxy.md) | Panel routing nginx → gosite | ✅ |

## Auth & monitoring

| # | File | Feature | Status |
|---|------|-------|--------|
| 03 | [03-authentication.md](./03-authentication.md) | Basic auth + session + lockscreen | ✅ |
| 04 | [04-dashboard.md](./04-dashboard.md) | Dashboard aggregate + system APIs | ✅ |

## Website, nginx, SSL

| # | File | Feature | Status |
|---|------|-------|--------|
| 05 | [05-website-create.md](./05-website-create.md) | Create + validate dry-run | ✅ |
| 06 | [06-website-enable-disable.md](./06-website-enable-disable.md) | Toggle + reload + repair | ✅ |
| 07 | [07-website-nginx-config.md](./07-website-nginx-config.md) | Edit & test nginx config | ✅ |
| 08 | [08-website-ssl.md](./08-website-ssl.md) | Certbot job + SSE, manual SSL | ✅ |
| 09 | [09-website-delete.md](./09-website-delete.md) | Delete + clean flag | ✅ |
| — | [../nginx-repair.md](../nginx-repair.md) | Auto-repair sebelum reload | ✅ |

## Server operations

| # | File | Feature | Status |
|---|------|-------|--------|
| 10 | [10-docker.md](./10-docker.md) | Docker Engine API | ✅ |
| 11 | [11-file-manager.md](./11-file-manager.md) | Files + batch ops | ✅ |
| 12 | [12-mount-manager.md](./12-mount-manager.md) | fstab + S3 secrets | ✅ |
| 13 | [13-cron-jobs.md](./13-cron-jobs.md) | Scheduler + SSE manual run | ✅ |
| 14 | [14-settings.md](./14-settings.md) | Profile only (PHP dropped) | ✅ |
| 15 | [15-logs.md](./15-logs.md) | Log tail viewer | ✅ |
| 16 | [16-database-viewer.md](./16-database-viewer.md) | SQLite read-only | ✅ |
| 17 | [17-splunk-lite.md](./17-splunk-lite.md) | Audit + log query | ✅ |
| 18 | [18-grafana-lite.md](./18-grafana-lite.md) | Traffic metrics | ✅ |
| 19 | [19-plugin-installer.md](./19-plugin-installer.md) | Plugin installer + compatibility contract | ✅ |
| 20 | [20-plugin-remote-distribution.md](./20-plugin-remote-distribution.md) | Remote install (GitHub/GitLab/URL) — wave G | ✅ |

## Wiki GitHub

Wiki page layout: [../wiki.md](../wiki.md).

## Cara pakai

1. Read the relevant sequence module
2. Cross-check with [api-inventory.md](../api-inventory.md) and `api/openapi.yaml`
3. For nginx/SSL: [nginx-repair.md](../nginx-repair.md)
