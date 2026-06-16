# Sequence: Enable / Disable Website

Toggle active website via symlink between `site.d` and `active.d`, then nginx loads active vhosts.

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
    alt missing
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

## How nginx loads active sites

```nginx
# nginx.conf
include /storage/webconfig/active.d/*.conf;
```

Only files in `active.d/` are served as additional vhosts.

## Notes

- Toggle **calls `nginx reload`** (with test + auto-repair) — improvement over legacy
- See [nginx-repair.md](../nginx-repair.md) for fallback when config is broken

## GoSite

```
PATCH /api/v1/websites/{id}/toggle
```

Response:
```json
{ "id": 1, "active": true, "message": "Site actived successfully" }
```

Service:
1. Update DB `active`
2. Symlink/unlink `active.d`
3. `nginx.Service.Reload()` → `TestAndRepair` + `nginx -s reload`
