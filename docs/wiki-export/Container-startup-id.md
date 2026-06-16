> **English:** [Container-startup](Container-startup)


Proses saat container GoSite pertama kali (atau restart) dijalankan.

**Entrypoint:** `config/start.sh` → nginx + `gosite serve`

## GoSite (implementasi saat ini)

```mermaid
sequenceDiagram
    actor Docker
    participant Start as start.sh
    participant Init as gosite init
    participant Repair as gosite nginx-repair
    participant SSL as openssl (default cert)
    participant Fstab as fstab_mounter.sh
    participant NGX as nginx
    participant Go as gosite serve

    Docker->>Start: CMD start.sh

    Start->>Start: mkdir /storage/logs, /storage/www
    Start->>Init: gosite init
    Note over Init: storage layout, symlink,<br/>migrate, seed admin/cron/demo

    alt default SSL belum ada
        Start->>SSL: self-signed cert.pem + key.pem
    end

    Start->>Repair: gosite nginx-repair
    Note over Repair: nginx -t + auto-fix aman

    opt /var/setup staging
        Start->>Start: mv nginx → /etc/nginx, copy webconfig
    end

    Start->>Start: substitute __PUBLIC_HTTPS_PORT__ di nginx conf
    Start->>Fstab: /run/fstab_mounter.sh
    Start->>NGX: nginx -c /etc/nginx/nginx.conf
    Start->>Go: exec gosite serve

    Note over Go: job worker + nginx watchdog (30s)
```

### Proses runtime

| Proses | Command | Catatan |
|--------|---------|---------|
| nginx | `nginx -c /etc/nginx/nginx.conf` | Di-start dari `start.sh`; reload/restart via Go |
| gosite | `gosite serve` | PID 1; watchdog start ulang nginx jika mati |

Cron job renewal & manual run dikelola **job worker** di dalam proses `gosite serve`, bukan proses PHP terpisah.

### `gosite init` (bootstrap)

| Langkah | Output |
|---------|--------|
| `createStorageLayout` | `/storage/webconfig`, `site.d`, `active.d`, `logs`, … |
| `copyTemplatesIfMissing` | Template dari image → storage |
| `createSymlinks` | `/etc/nginx` → `/storage/nginx`, `/etc/letsencrypt` → `/storage/webconfig/ssl`, `/www` → `/storage/www` |
| `sqlite.Migrate` | Schema `db.sqlite` |
| `seedAdminIfEmpty` | User demo |
| `seedDefaultCronIfEmpty` | `certbot renew --post-hook 'nginx -s reload'` |
| `seedDemoIfNeeded` | Website demo (jika `DEMO_SEED=true`) |

### Boot nginx repair

`gosite nginx-repair` dijalankan **setelah** default SSL dibuat agar fallback repair bisa mengarahkan vhost ke cert default. Lihat [nginx-repair.md](Nginx-auto-repair-id).

---


## Invariant produksi

- Struktur `/storage` kompatibel dengan deploy BangunSite lama
- Symlink `/etc/letsencrypt` → `/storage/webconfig/ssl` — path yang sama dipakai Certbot dan placeholder Gosite
