# Sequence: Buat Website

Membuat entri website baru: validasi → simpan DB → generate nginx config → notifikasi email.

**Route:** `POST /admin/website` → `WebsiteManagerController@store`

```mermaid
sequenceDiagram
    actor User
    participant WM as WebsiteManagerController
    participant Val as check()
    participant DB as websites
    participant Site as Site library
    participant Disk
    participant Cmd as Commander
    participant Mail

    User->>WM: POST /admin/website { domain, path, name?, active? }
    WM->>Val: validate domain & path
    alt domain invalid / path illegal / path dipakai / path is file
        Val-->>WM: error message
        WM-->>User: redirect back + error
    end
    WM->>DB: Website::create()
    alt create gagal
        WM-->>User: error
    end
    Note over Site: createConfig dipanggil lazy saat getSiteConfig
    WM->>Site: (implisit) createConfig saat config pertama dibaca
    Site->>Disk: mkdir path, copy index.html default
    Site->>Cmd: chown apps:apps path
    Site->>Disk: write site.d/{domain}.conf dari template site.conf
    opt MAIL_NOTIFICATION
        WM->>Mail: Created Notification
    end
    WM-->>User: success redirect
```

## createConfig detail

**Library:** `Site::createConfig()`

```mermaid
sequenceDiagram
    participant Site
    participant Template as site.conf
    participant SSL as ssl/live/default
    participant FS as filesystem

    Site->>Template: baca placeholder <domain>, <path>, <ssl_*>
    Site->>SSL: copySSL → ssl/live/{domain}/cert.pem, key.pem
    Site->>FS: mkdir(path), copy index.html
    Site->>FS: write /storage/webconfig/site.d/{domain}.conf
```

## Validasi `check()`

| Check | Hasil |
|-------|-------|
| `FILTER_VALIDATE_DOMAIN` | domain valid |
| `Disk::validatePath()` | path aman |
| Path unik di DB | tidak dipakai site lain |
| `is_file(path)` | ditolak |

## Implikasi GoSite

```
POST /api/v1/websites
POST /api/v1/websites/validate   # optional pre-flight
```

Side-effect yang harus di service layer:
1. Insert SQLite
2. Generate `site.d/{domain}.conf`
3. Copy default SSL + index.html
4. **Belum** symlink `active.d` kecuali `active=true` saat create
