# Sequence: Hapus Website

**Route:** `DELETE /admin/website/{domain}/enableSite` → `destroy()`

```mermaid
sequenceDiagram
    actor User
    participant WM as WebsiteManagerController
    participant DB as websites
    participant Site as Site library
    participant Ngx as Nginx
    participant FS as filesystem
    participant Mail

    User->>WM: DELETE /admin/website/{domain}/enableSite?clean=
    WM->>DB: Website::getSite(domain)
    alt tidak ada
        WM-->>User: 400 JSON
    end
    WM->>Site: removeSite(model, clean)
    Site->>Site: enableSite(domain, false) — unlink active.d
    Site->>Ngx: restart()
    opt clean=true dan path exists
        Site->>FS: Disk::rm(path, recursive)
    end
    Site->>FS: unlink site.d/{domain}.conf
    WM->>DB: delete()
    opt MAIL_NOTIFICATION
        WM->>Mail: Deleted Notification
    end
    WM-->>User: 200 { msg: "Site deleted successfully" }
```

## Parameter `clean`

| clean | Efek |
|-------|------|
| true / ada | Hapus document root (`path`) rekursif |
| false / null | Hanya hapus config & DB, biarkan files |

## Yang dihapus

1. Symlink `active.d/{domain}.conf`
2. File `site.d/{domain}.conf`
3. Record SQLite
4. (Opsional) folder `/www/...`

**Tidak dihapus otomatis:** sertifikat SSL di `ssl/live/{domain}/`, log files.

## Implikasi GoSite

```
DELETE /api/v1/websites/{id}?clean=true
```

Urutan aman:
1. Disable site
2. Nginx reload
3. Hapus config files
4. Hapus path jika diminta
5. Hapus DB record

Pertimbangkan konfirmasi UI untuk `clean=true`.
