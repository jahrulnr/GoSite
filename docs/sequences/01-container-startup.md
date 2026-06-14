# Sequence: Container Startup

Proses saat container `bangunsite` pertama kali (atau restart) dijalankan.

**Entrypoint:** `config/start.sh` → `supervisord`

```mermaid
sequenceDiagram
    actor Docker
    participant Start as start.sh
    participant Storage as /storage
    participant Composer
    participant Artisan as Laravel Artisan
    participant Fstab as fstab_mounter.sh
    participant Super as supervisord

    Docker->>Start: CMD /run/start.sh

    Start->>Storage: mkdir logs/, www/, webconfig/, nginx/
    alt webconfig belum ada
        Start->>Storage: cp /var/setup/webconfig → /storage/webconfig
    end
    alt nginx/php belum ada
        Start->>Storage: cp /var/setup/nginx, php
    end
    Start->>Storage: symlink .env, fstab, nginx, php, www
    Start->>Storage: chown apps:apps

    alt vendor belum ada
        Start->>Composer: composer install --no-dev
    end
    alt .env belum ada
        Start->>Storage: cp .env.example → /storage/.env
        Start->>Artisan: key:generate
    end
    alt db.sqlite belum ada
        Start->>Storage: touch db.sqlite
        Start->>Artisan: migrate --force
        Start->>Artisan: db:seed --force
    end
    alt SSL default belum ada
        Start->>Storage: make build (self-signed cert)
    end

    Start->>Fstab: mount semua entry /etc/fstab
    Start->>Super: supervisord -n

    Super->>Super: start nginx (:80/:443)
    Super->>Super: start artisan server (:8000)
    Super->>Super: start run:cronjobs
    Super->>Super: start server-proxy (:8080)
```

## Supervisor programs

| Program | Command | Prioritas |
|---------|---------|-----------|
| nginx | `nginx -g "daemon off;"` | 10 |
| bangunsite | `php artisan server --host=0.0.0.0 --port=8000` | 15 |
| crond | `php artisan run:cronjobs` | 16 |
| proxy-server | `/usr/bin/server-proxy` | 20 |

## Implikasi GoSite

- Startup script bisa tetap bash atau diganti binary `gosite init`
- Proses `artisan server` + `server-proxy` diganti satu binary Go (HTTP + HTTPS)
- Nginx & cron runner tetap proses terpisah atau dikelola Go supervisor
- Invariant: struktur `/storage` harus kompatibel agar data produksi bisa dipakai
