# Sequence: Enable / Disable Website

Toggle website aktif dengan symlink antara `site.d` dan `active.d`, lalu nginx memuat vhost aktif.

**Route:** `OPTIONS /admin/website/{id}/enableSite` → `enableSite()`

```mermaid
sequenceDiagram
    actor User
    participant WM as WebsiteManagerController
    participant DB as websites
    participant Site as Site library
    participant FS as active.d / site.d

    User->>WM: OPTIONS /admin/website/{domain}/enableSite
    WM->>DB: Website::getSite(domain)
    alt tidak ada
        WM-->>User: 400 JSON error
    end
    WM->>WM: site.active = !site.active
    WM->>Site: enableSite(domain, active)

    alt enable=true
        Site->>FS: symlink site.d/{domain}.conf → active.d/{domain}.conf
    else enable=false
        Site->>FS: unlink active.d/{domain}.conf
    end

    WM->>DB: save()
    WM-->>User: 200 JSON { msg: "actived/disabled successfully" }
```

## Bagaimana nginx memuat site aktif

```nginx
# nginx.conf
include /storage/webconfig/active.d/*.conf;
```

Hanya file di `active.d/` yang dilayani sebagai vhost tambahan.

## Catatan

- Toggle **tidak** otomatis `nginx reload` di legacy (perlu reload manual atau via config update)
- Di praktik produksi, perubahan `active.d` biasanya butuh `nginx -s reload`

## Implikasi GoSite

```
PATCH /api/v1/websites/{id}/toggle
```

Response:
```json
{ "id": 1, "active": true, "message": "Site actived successfully" }
```

Service harus:
1. Update DB `active`
2. Symlink/unlink
3. **Panggil `nginx reload`** (perbaikan dari legacy)
