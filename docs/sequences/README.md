# Sequence Diagrams — Index

Setiap file berisi diagram Mermaid berdasarkan implementasi aktual di `/apps/profile/bangunsite`.

## Runtime & infrastruktur

| # | File | Fitur |
|---|------|-------|
| 01 | [01-container-startup.md](./01-container-startup.md) | First boot & supervisor |
| 02 | [02-tls-proxy.md](./02-tls-proxy.md) | Go proxy panel :8080 |

## Auth & monitoring

| # | File | Fitur |
|---|------|-------|
| 03 | [03-authentication.md](./03-authentication.md) | Basic auth, login, lockscreen |
| 04 | [04-dashboard.md](./04-dashboard.md) | Dashboard + API polling |

## Website, nginx, SSL

| # | File | Fitur |
|---|------|-------|
| 05 | [05-website-create.md](./05-website-create.md) | Buat website + generate vhost |
| 06 | [06-website-enable-disable.md](./06-website-enable-disable.md) | Toggle active.d symlink |
| 07 | [07-website-nginx-config.md](./07-website-nginx-config.md) | Edit & test nginx config |
| 08 | [08-website-ssl.md](./08-website-ssl.md) | Certbot & manual SSL |
| 09 | [09-website-delete.md](./09-website-delete.md) | Hapus site |

## Operasional server

| # | File | Fitur |
|---|------|-------|
| 10 | [10-docker.md](./10-docker.md) | Kelola container |
| 11 | [11-file-manager.md](./11-file-manager.md) | Browse & manipulasi file |
| 12 | [12-mount-manager.md](./12-mount-manager.md) | fstab & mount |
| 13 | [13-cron-jobs.md](./13-cron-jobs.md) | Scheduler & manual run |
| 14 | [14-settings.md](./14-settings.md) | Profile & PHP/FPM |
| 15 | [15-logs.md](./15-logs.md) | Log viewer |
| 16 | [16-database-viewer.md](./16-database-viewer.md) | SQLite admin |
| 17 | [17-splunk-lite.md](./17-splunk-lite.md) | Splunk Lite query |
| 18 | [18-grafana-lite.md](./18-grafana-lite.md) | Grafana Lite metrics |

## Cara pakai untuk migrasi

1. Baca sequence modul yang akan diimplementasi di Go
2. Cocokkan dengan endpoint di [api-inventory.md](../api-inventory.md)
3. Tandai side-effect OS (shell, symlink, reload) di infrastructure layer
4. Frontend nanti hanya mengikuti API — tidak perlu tahu nginx/certbot detail
