# Sequence: Container Startup

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
    Start->>Start: migrasi log legacy → /storage/logs
    Start->>Start: seed /var/setup/nginx → /storage/nginx (cp -a -n)
    Start->>Init: gosite init
    Note over Init: storage layout, symlink,<br/>migrate, seed admin/cron/demo

    alt default SSL belum ada
        Start->>SSL: self-signed cert.pem + key.pem
    end

    Start->>Start: siapkan /www/default/index.html

    opt /var/setup staging
        Start->>Start: webconfig → /storage/webconfig (cp -a -n), hapus /var/setup
    end

    Start->>Repair: gosite nginx-repair
    Note over Repair: nginx -t + auto-fix aman

    Start->>Start: substitute __PUBLIC_HTTPS_PORT__ di nginx conf
    Start->>Fstab: /run/fstab_mounter.sh
    Start->>NGX: nginx -c /etc/nginx/nginx.conf
    Start->>Start: daemon logrotate
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
| `createStorageLayout` | `/storage/webconfig`, `site.d`, `active.d`, `logs`, `nginx`, … (+ migrasi log legacy Laravel) |
| `copyTemplatesIfMissing` | Merge `/var/setup/{webconfig,nginx}` → `/storage`: file yang belum ada disalin (healing), file persisten tidak pernah ditimpa |
| `createSymlinks` | `/etc/nginx` → `/storage/nginx`, `/etc/letsencrypt` → `/storage/webconfig/ssl`, `/www` → `/storage/www` |
| `sqlite.Migrate` | Schema `db.sqlite` |
| `seedAdminIfEmpty` | User demo |
| `seedDefaultCronIfEmpty` | `certbot renew --post-hook 'nginx -s reload'` |
| `seedDemoIfNeeded` | Website demo (jika `DEMO_SEED=true`) |

Jika `/etc/nginx` masih berupa direktori nyata (bukan symlink), `createSymlinks` memigrasikan isinya ke `/storage/nginx` tanpa menimpa file yang sudah ada; entry konflik disimpan di `/storage/nginx.migrated-conflicts/`. Entry top-level `conf.d` bawaan image dikarantina ke `/storage/nginx.migrated-conflicts/conf.d/` supaya `listen 80 default_server` bawaannya tidak bentrok dengan vhost persisten.

### Kepemilikan config nginx

`/storage/nginx` adalah sumber kebenaran persisten untuk `/etc/nginx` — di-seed dari `/var/setup/nginx` dengan `cp -a -n` oleh `start.sh` sebelum `gosite init`. File template dari image hanya ditambahkan jika belum ada, jadi **update template di image tidak menimpa editan persisten** — rekonsiliasi manual diperlukan saat upgrade.

### Boot nginx repair

`gosite nginx-repair` dijalankan setelah staging `/var/setup` dibersihkan dan **sebelum** nginx start — juga setelah default SSL dibuat agar fallback repair bisa mengarahkan vhost ke cert default. Lihat [nginx-repair.md](../operations/nginx-repair_id.md).

---

## Legacy BangunSite (referensi migrasi)

<details>
<summary>Diagram startup Laravel (historis)</summary>

```mermaid
sequenceDiagram
    participant Start as start.sh
    participant Composer
    participant Artisan as Laravel Artisan
    participant Super as supervisord

    Start->>Composer: composer install
    Start->>Artisan: migrate + db:seed
    Start->>Super: supervisord
    Super->>Super: nginx + artisan server + cron + server-proxy :8080
```

| Program | Command |
|---------|---------|
| bangunsite | `php artisan server --port=8000` |
| proxy-server | Go TLS proxy :8080 |
| crond | `php artisan run:cronjobs` |

</details>

## Invariant produksi

- Struktur `/storage` kompatibel dengan deploy BangunSite lama
- Symlink `/etc/letsencrypt` → `/storage/webconfig/ssl` — path yang sama dipakai Certbot dan placeholder Gosite
