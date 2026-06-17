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
    Start->>Init: gosite init
    Note over Init: storage layout, symlink,<br/>migrate, seed admin/cron/demo

    alt default SSL missing
        Start->>SSL: self-signed cert.pem + key.pem
    end

    Start->>Repair: gosite nginx-repair
    Note over Repair: nginx -t + safe auto-fix

    opt /var/setup staging
        Start->>Start: mv nginx → /etc/nginx, copy webconfig
    end

    Start->>Start: substitute __PUBLIC_HTTPS_PORT__ in nginx conf
    Start->>Fstab: /run/fstab_mounter.sh
    Start->>NGX: nginx -c /etc/nginx/nginx.conf
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
| `createStorageLayout` | `/storage/webconfig`, `site.d`, `active.d`, `logs`, … |
| `copyTemplatesIfMissing` | Templates from image → storage |
| `createSymlinks` | `/etc/nginx` → `/storage/nginx`, `/etc/letsencrypt` → `/storage/webconfig/ssl`, `/www` → `/storage/www` |
| `sqlite.Migrate` | Schema `db.sqlite` |
| `seedAdminIfEmpty` | User demo |
| `seedDefaultCronIfEmpty` | `certbot renew --post-hook 'nginx -s reload'` |
| `seedDemoIfNeeded` | Demo website (when `DEMO_SEED=true`) |

### Boot nginx repair

`gosite nginx-repair` runs **after** the default SSL cert is created so fallback repair can point vhosts to the default cert. See [nginx-repair.md](../operations/nginx-repair.md).

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
