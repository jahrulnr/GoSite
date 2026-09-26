# Sequence: Container Startup

What happens when the GoSite container starts (first boot or restart).

**Entrypoint:** `config/start.sh` → nginx + `gosite serve`

## GoSite (current implementation)

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
    Start->>Start: migrate legacy logs → /storage/logs
    Start->>Start: seed /var/setup/nginx → /storage/nginx (cp -a -n)
    Start->>Init: gosite init
    Note over Init: storage layout, symlink,<br/>migrate, seed admin/cron/demo

    alt default SSL missing
        Start->>SSL: self-signed cert.pem + key.pem
    end

    Start->>Start: prepare /www/default/index.html

    opt /var/setup staging
        Start->>Start: webconfig → /storage/webconfig (cp -a -n), remove /var/setup
    end

    Start->>Repair: gosite nginx-repair
    Note over Repair: nginx -t + safe auto-fix

    Start->>Start: substitute __PUBLIC_HTTPS_PORT__ in nginx conf
    Start->>Fstab: /run/fstab_mounter.sh
    Start->>NGX: nginx -c /etc/nginx/nginx.conf
    Start->>Start: logrotate daemon
    Start->>Go: exec gosite serve

    Note over Go: job worker + nginx watchdog (30s)
```

### Runtime processes

| Process | Command | Notes |
|--------|---------|---------|
| nginx | `nginx -c /etc/nginx/nginx.conf` | Started from `start.sh`; reload/restart via Go |
| gosite | `gosite serve` | PID 1; watchdog restarts nginx if it dies |

Cron renewal and manual runs are handled by the **job worker** inside `gosite serve`, not a separate PHP process.

### `gosite init` (bootstrap)

| Step | Output |
|---------|--------|
| `createStorageLayout` | `/storage/webconfig`, `site.d`, `active.d`, `logs`, `nginx`, … (+ legacy Laravel log migration) |
| `copyTemplatesIfMissing` | Merge `/var/setup/{webconfig,nginx}` → `/storage`: missing files are copied (healing), persisted files are never overwritten |
| `createSymlinks` | `/etc/nginx` → `/storage/nginx`, `/etc/letsencrypt` → `/storage/webconfig/ssl`, `/www` → `/storage/www` |
| `sqlite.Migrate` | Schema `db.sqlite` |
| `seedAdminIfEmpty` | User demo |
| `seedDefaultCronIfEmpty` | `certbot renew --post-hook 'nginx -s reload'` |
| `seedDemoIfNeeded` | Demo website (when `DEMO_SEED=true`) |

If `/etc/nginx` is still a real directory (not a symlink), `createSymlinks` migrates its contents into `/storage/nginx` without overwriting existing files; conflicting entries are kept under `/storage/nginx.migrated-conflicts/`. A top-level `conf.d` entry from the image is quarantined under `/storage/nginx.migrated-conflicts/conf.d/` so its `listen 80 default_server` cannot collide with persisted vhosts.

### Nginx config ownership

`/storage/nginx` is the persistent source of truth for `/etc/nginx` — seeded from `/var/setup/nginx` with `cp -a -n` by `start.sh` before `gosite init`. Template files from the image are only added when missing, so **image template updates never overwrite persisted edits** — manual reconciliation is required on upgrade.

### Boot nginx repair

`gosite nginx-repair` runs after the `/var/setup` staging area is cleaned up and **before** nginx starts — also after the default SSL cert is created so fallback repair can point vhosts to the default cert. See [nginx-repair.md](../operations/nginx-repair.md).

---

## Legacy BangunSite (migration reference)

<details>
<summary>Historical Laravel startup diagram</summary>

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

## Production invariants

- `/storage` layout compatible with legacy BangunSite deploy
- Symlink `/etc/letsencrypt` → `/storage/webconfig/ssl` — same path used by Certbot and Gosite placeholders
