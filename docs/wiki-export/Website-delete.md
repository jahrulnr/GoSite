> **Bahasa Indonesia:** [Website-delete-id](Website-delete-id)


**API:** `DELETE /api/v1/websites/{id}?clean=true|false`

## GoSite (implementation)

```mermaid
sequenceDiagram
    actor User
    participant API as WebsiteHandler
    participant Svc as website.Service
    participant DB as SQLite
    participant NGX as nginx.Service
    participant FS as filesystem

    User->>API: DELETE /websites/{id}?clean=
    API->>Svc: Delete(id, clean)
    Svc->>NGX: DisableSite — unlink active.d
    Svc->>NGX: Reload (TestAndRepair + reload)
    opt clean=true
        Svc->>FS: RemoveAll(site.Path)
    end
    Svc->>NGX: RemoveSiteConfig — remove site.d
    Svc->>DB: DELETE websites
    API-->>User: 200 { message }
```

### Parameter `clean`

| clean | Efek |
|-------|------|
| `true` | Hapus document root (`path`) rekursif |
| `false` / omitted | Keep `/www/...` folder, remove config + DB only |

UI shows confirmation before delete.

### What is removed

1. Symlink `active.d/{domain}.conf`
2. File `site.d/{domain}.conf`
3. Record SQLite

### Not removed automatically

- Sertifikat `ssl/live/{domain}/`
- File log `access-{domain}.log`, `error-{domain}.log`

### Safe order

1. Disable + reload nginx (vhost no longer active)
2. Hapus path jika `clean=true`
3. Hapus `site.d` config
4. Hapus baris DB

Reload uses [nginx auto-repair](Nginx-auto-repair) when other config is broken.

---

