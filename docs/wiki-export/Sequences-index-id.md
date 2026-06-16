> **English:** [Sequences-index](Sequences-index)


Diagram alur fitur GoSite. Bagian **Legacy BangunSite** (jika ada) disimpan sebagai referensi migrasi.

## Runtime & infrastruktur

| # | File | Fitur | Status |
|---|------|-------|--------|
| 01 | [01-container-startup.md](Container-startup-id) | `start.sh`, `gosite init`, nginx-repair | ✅ |
| 02 | [02-tls-proxy.md](Panel-routing-id) | Panel routing nginx → gosite | ✅ |

## Auth & monitoring

| # | File | Fitur | Status |
|---|------|-------|--------|
| 03 | [03-authentication.md](Authentication-id) | Basic auth + session + lockscreen | ✅ |
| 04 | [04-dashboard.md](Dashboard-id) | Dashboard aggregate + system APIs | ✅ |

## Website, nginx, SSL

| # | File | Fitur | Status |
|---|------|-------|--------|
| 05 | [05-website-create.md](Website-create-id) | Create + validate dry-run | ✅ |
| 06 | [06-website-enable-disable.md](Website-enable-disable-id) | Toggle + reload + repair | ✅ |
| 07 | [07-website-nginx-config.md](Website-nginx-config-id) | Edit & test nginx config | ✅ |
| 08 | [08-website-ssl.md](SSL-and-Certbot-id) | Certbot job + SSE, manual SSL | ✅ |
| 09 | [09-website-delete.md](Website-delete-id) | Delete + clean flag | ✅ |
| — | [../nginx-repair.md](Nginx-auto-repair-id) | Auto-repair sebelum reload | ✅ |

## Operasional server

| # | File | Fitur | Status |
|---|------|-------|--------|
| 10 | [10-docker.md](Operations-id) | Docker Engine API | ✅ |
| 11 | [11-file-manager.md](Operations-id) | Files + batch ops | ✅ |
| 12 | [12-mount-manager.md](Operations-id) | fstab + S3 secrets | ✅ |
| 13 | [13-cron-jobs.md](Operations-id) | Scheduler + SSE manual run | ✅ |
| 14 | [14-settings.md](Operations-id) | Profile only (PHP dropped) | ✅ |
| 15 | [15-logs.md](Operations-id) | Log tail viewer | ✅ |
| 16 | [16-database-viewer.md](Operations-id) | SQLite read-only | ✅ |
| 17 | [17-splunk-lite.md](Observability-id) | Audit + log query | ✅ |
| 18 | [18-grafana-lite.md](Observability-id) | Traffic metrics | ✅ |

## Wiki GitHub

Struktur halaman wiki: [../wiki.md](../wiki_id.md).

## Cara pakai

1. Baca sequence modul yang relevan
2. Cocokkan dengan [api-inventory.md](../api-inventory_id.md) dan `api/openapi.yaml`
3. Untuk nginx/SSL: [nginx-repair.md](Nginx-auto-repair-id)
