# Sequence: File Manager

Browser dan manipulasi file di server. Default root: `WEB_PATH` (`/www`).

**Base route:** `/admin/browse`

## Browse directory

```mermaid
sequenceDiagram
    actor User
    participant FM as FileManagerController
    participant Disk

    User->>FM: GET /admin/browse?path=/www/foo
    FM->>FM: resolve path (handle .. traversal)
    alt path tidak ada
        FM-->>User: redirect + error
    end
    FM->>Disk: ls(path)
    FM-->>User: Blade listing
```

## Read file content

```mermaid
sequenceDiagram
    actor User
    participant FM as FileManagerController

    User->>FM: POST /admin/browse/show { name }
    FM->>FM: path + name → file path
    alt bukan file
        FM-->>User: 400 JSON error
    end
    FM-->>User: JSON { status, content }
```

## Create (directory / file / remote / upload)

```mermaid
sequenceDiagram
    actor User
    participant FM as FileManagerController
    participant Disk
    participant Shell as shell_exec

    User->>FM: POST /admin/browse/new { type, name, path, ... }

    alt type=directory
        FM->>Shell: mkdir -p && chmod && chown (if under /www)
    else type=file
        FM->>Disk: createFile(content)
        FM->>Shell: chmod + chown
    else type=remote
        FM->>Disk: curl(url) → createFile
    else type=upload
        FM->>FM: validate upload_max_filesize
        FM->>FM: file->move(dest)
    end
    FM-->>User: redirect / JSON success
```

## Actions: chmod, copy, execute

| type | Aksi |
|------|------|
| chmod | `chmod {perm} {path}` — validasi 600–777 |
| copy | `Disk::cp` ke `toPath`, mkdir jika perlu |
| execute | `nohup {path} &` — butuh permission ≥ 775, bukan folder |

## Delete

```mermaid
sequenceDiagram
    actor User
    participant FM as FileManagerController
    participant Disk

    User->>FM: DELETE /admin/browse/action { path, name }
    FM->>Disk: rm(path, recursive=true)
    FM-->>User: success redirect
```

## Implikasi GoSite

| Endpoint | Catatan |
|----------|---------|
| `GET /files?path=` | Listing dengan metadata (name, size, perm, is_dir) |
| `GET /files/content` | Baca teks/binary base64 |
| `POST /files` | Multipart upload |
| `POST /files/actions` | chmod, copy, execute |
| `DELETE /files` | Hapus |

**Keamanan wajib:**
- Allowlist root: `/www`, `/storage`, `/tmp` (konfigurasi)
- Tolak `..` dan path absolut di luar allowlist
- Execute command: sangat terbatas atau dinonaktifkan di produksi
