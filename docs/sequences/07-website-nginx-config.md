# Sequence: Edit Nginx Config

Tiga level konfigurasi nginx di BangunSite.

## A. Config per website

**Route:** `POST /admin/website/{id}/updateConfig`

```mermaid
sequenceDiagram
    actor User
    participant WM as WebsiteManagerController
    participant Disk
    participant Ngx as Nginx library
    participant Super as supervisorctl

    User->>WM: POST updateConfig { config }
    WM->>Disk: backup oriConfig, write site.d/{domain}.conf
    WM->>Ngx: test(domain) — nginx -t dengan hanya file domain
    alt syntax ok
        Ngx->>Super: restart nginx
        WM-->>User: success
    else syntax error
        WM->>Disk: restore oriConfig
        WM-->>User: error + config draft
    end
```

## B. Default server config

**Route:** `POST /admin/website/default/updateConfig` (id = `default`)

- Target: `/etc/nginx/http.d/default.conf`
- Sama: test → restart atau rollback

## C. Global nginx.conf

**Route:** `PATCH /admin/website/updateNginx`

```mermaid
sequenceDiagram
    actor User
    participant WM as WebsiteManagerController
    participant Disk
    participant Ngx as Nginx library

    User->>WM: PATCH updateNginx { content }
    WM->>Disk: write nginx-test.conf
    WM->>Ngx: testNginxConf(tmp path)
    alt ok
        WM->>Disk: write nginx.conf
        WM->>Ngx: restart()
        WM-->>User: success
    else fail
        WM-->>User: error message dari nginx -t
    end
```

## Nginx::test(domain) detail

1. Clone `nginx.conf`, ganti include `site.d/*.conf` → `site.d/{domain}.conf` saja
2. Jalankan `nginx -t -c /tmp/nginx-{time}.conf`
3. Hapus file tmp

## Implikasi GoSite

| Endpoint | File target |
|----------|-------------|
| `PUT /websites/{id}/nginx-config` | `site.d/{domain}.conf` |
| `PUT /nginx/default` | `http.d/default.conf` |
| `PUT /nginx/global` | `nginx.conf` |
| `POST /nginx/test` | body config → `{ ok: true }` atau error |

Invariant: **selalu test sebelum apply + rollback on failure**.
