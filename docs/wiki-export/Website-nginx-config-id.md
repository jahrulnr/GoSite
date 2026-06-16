> **English:** [Website-nginx-config](Website-nginx-config)


Tiga level konfigurasi nginx di GoSite.

## A. Config per website

**API:** `PUT /api/v1/websites/{id}/nginx-config`

```mermaid
sequenceDiagram
    actor User
    participant API as WebsiteHandler
    participant Svc as website.Service
    participant NGX as nginx.Service

    User->>API: PUT { config }
    API->>Svc: UpdateNginxConfig(id, config)
    Svc->>NGX: BackupSiteConfig
    Svc->>NGX: WriteSiteConfig (site.d)
    Svc->>NGX: TestConfig (temp file, tidak rewrite site.d)
    alt test ok
        Svc->>NGX: Reload (TestAndRepair + reload)
        API-->>User: success
    else test fail
        Svc->>NGX: restore backup
        API-->>User: NGINX_TEST_FAILED
    end
```

## B. Default server config

**API:** `GET/PUT /api/v1/nginx/default`  
**File:** `/etc/nginx/http.d/default.conf`

Alur: `TestDefaultConfig` (temp + clone nginx.conf) → write → `Reload`.

## C. Global nginx.conf

**API:** `GET/PUT /api/v1/nginx/global`  
**File:** `/etc/nginx/nginx.conf`

Sama: test raw content di temp → write → reload.

## Nginx test per domain (`TestConfig`)

Digunakan oleh: validate website, update site config, create (active).

```mermaid
flowchart LR
    A[Render / input content] --> B[Write /tmp/nginx-site-test-*.conf]
    B --> C[Clone webconfig/nginx.conf]
    C --> D["Replace include site.d/*.conf → path temp absolut"]
    D --> E["nginx -t -c /tmp/nginx-test-*.conf"]
    E --> F[Delete temp files]
```

**Penting:** penggantian include harus memakai path **absolut** ke file temp. Mengganti hanya bagian `site.d/*.conf` menghasilkan path invalid (`/storage/webconfig//tmp/...`).

File test terisolasi: `config/webconfig/nginx.conf` — hanya memuat satu vhost, tanpa `http.d/default.conf`.

## Reload & auto-repair

Setiap `Reload()` memanggil `TestAndRepair` pada config produksi penuh sebelum `nginx -s reload`. Lihat [nginx-repair.md](Nginx-auto-repair-id).

## API ringkas

| Method | Path | File target |
|--------|------|-------------|
| PUT | `/websites/{id}/nginx-config` | `site.d/{domain}.conf` |
| GET | `/websites/{id}/nginx-config` | baca `site.d` |
| PUT | `/nginx/default` | `http.d/default.conf` |
| PUT | `/nginx/global` | `nginx.conf` |
| POST | `/nginx/reload` | reload + repair |
| POST | `/nginx/test` | test body arbitrary |

Invariant: **test sebelum apply + rollback on failure** (update site config); **repair + test sebelum reload** (semua reload).
