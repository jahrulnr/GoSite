# Sequence: SSL Management

Dua jalur: **Certbot otomatis** dan **upload manual**.

## A. Install SSL via Certbot (async)

**Route:** `GET /admin/website/{id}/installSSL?start=true`

```mermaid
sequenceDiagram
    actor User
    participant WM as WebsiteManagerController
    participant Disk
    participant Queue as RunCommand job
    participant Certbot
    participant Tmp as /tmp/{domain}.txt

    User->>WM: GET installSSL?start=true
    WM->>Disk: write /tmp/{domain}.sh (certbot --nginx -d {domain})
    WM->>Queue: dispatch RunCommand(script > output)
    WM-->>User: "Waiting task on queue"

    loop polling
        User->>WM: GET installSSL (start=false)
        alt output file exists
            WM->>Tmp: read output
            WM-->>User: certbot log text
        else belum selesai
            WM-->>User: "Waiting task on queue"
        end
    end

    Queue->>Certbot: certbot --non-interactive --nginx -d domain
    Certbot-->>Tmp: stdout/stderr
```

**Command:** `certbot --non-interactive --agree-tos --register-unsafely-without-email --nginx -d {domain}`

## B. Manual SSL upload

**Route:** `POST /admin/website/{id}/updateSSL`

```mermaid
sequenceDiagram
    actor User
    participant WM as WebsiteManagerController
    participant SSL as SSL library
    participant Disk
    participant FS as ssl/live + archive

    User->>WM: POST { public, private } PEM content
    WM->>SSL: getCertPath(domain)
    alt cert belum ada di config
        WM->>FS: write archive/{domain}/fullchain.pem, privkey.pem
        WM->>FS: symlink live/{domain}/
        Note over WM: setCustomSSL() — TODO/unimplemented (dd())
    else cert sudah ada
        WM->>FS: overwrite file di path dari config
    end
    WM-->>User: success / error write
```

## C. Baca status SSL

**Digunakan di halaman edit:**

- `SSL::readPublic(domain)` / `readPrivate(domain)` — parse path dari `site.d/{domain}.conf`
- `SSL::checkSSL(domain)` — cek directive `ssl_certificate` tidak di-comment

## Renewal otomatis (cron default)

```
certbot renew --post-hook 'supervisorctl restart nginx'
```

Jadwal: cronjob `day` — dijalankan oleh `artisan run:cronjobs`.

## Implikasi GoSite

| Endpoint | Keterangan |
|----------|------------|
| `POST /websites/{id}/ssl/certbot` | Mulai job |
| `GET /websites/{id}/ssl/certbot/stream` | SSE output |
| `PUT /websites/{id}/ssl/manual` | Upload PEM |
| `GET /websites/{id}/ssl` | Status + paths |

Job runner Go menggantikan Laravel Queue + file polling `/tmp/`.

Perbaikan yang disarankan: implement `setCustomSSL` — update directive di `site.d` + reload nginx.
