> **Bahasa Indonesia:** [Sequences-index-id](Sequences-index-id)


GoSite feature flow diagrams. **Legacy BangunSite** sections (if any) are kept as migration reference.

## Runtime & infrastructure

| # | File | Feature | Status |
|---|------|-------|--------|
| 01 | [01-container-startup.md](Container-startup) | `start.sh`, `gosite init`, nginx-repair | ✅ |
| 02 | [02-tls-proxy.md](Panel-routing) | Panel routing nginx → gosite | ✅ |

## Auth & monitoring

| # | File | Feature | Status |
|---|------|-------|--------|
| 03 | [03-authentication.md](Authentication) | Basic auth + session + lockscreen | ✅ |
| 04 | [04-dashboard.md](Dashboard) | Dashboard aggregate + system APIs | ✅ |

## Website, nginx, SSL

| # | File | Feature | Status |
|---|------|-------|--------|
| 05 | [05-website-create.md](Website-create) | Create + validate dry-run | ✅ |
| 06 | [06-website-enable-disable.md](Website-enable-disable) | Toggle + reload + repair | ✅ |
| 07 | [07-website-nginx-config.md](Website-nginx-config) | Edit & test nginx config | ✅ |
| 08 | [08-website-ssl.md](SSL-and-Certbot) | Certbot job + SSE, manual SSL | ✅ |
| 09 | [09-website-delete.md](Website-delete) | Delete + clean flag | ✅ |
| — | [../nginx-repair.md](Nginx-auto-repair) | Auto-repair sebelum reload | ✅ |

## Server operations

| # | File | Feature | Status |
|---|------|-------|--------|
| 10 | [10-docker.md](Operations) | Docker Engine API | ✅ |
| 11 | [11-file-manager.md](Operations) | Files + batch ops | ✅ |
| 12 | [12-mount-manager.md](Operations) | fstab + S3 secrets | ✅ |
| 13 | [13-cron-jobs.md](Operations) | Scheduler + SSE manual run | ✅ |
| 14 | [14-settings.md](Operations) | Profile only (PHP dropped) | ✅ |
| 15 | [15-logs.md](Operations) | Log tail viewer | ✅ |
| 16 | [16-database-viewer.md](Operations) | SQLite read-only | ✅ |
| 17 | [17-splunk-lite.md](Observability) | Audit + log query | ✅ |
| 18 | [18-grafana-lite.md](Observability) | Traffic metrics | ✅ |

## Wiki GitHub

Wiki page layout: [../wiki.md](Home).

## Cara pakai

1. Read the relevant sequence module
2. Cross-check with [api-inventory.md](API-reference) and `api/openapi.yaml`
3. For nginx/SSL: [nginx-repair.md](Nginx-auto-repair)
