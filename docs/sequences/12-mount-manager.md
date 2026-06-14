# Sequence: Mount Manager

Kelola `/etc/fstab` (symlink ke `/storage/fstab`) dan mount/umount filesystem.

## List mounts

**Route:** `GET /admin/mount`

```mermaid
sequenceDiagram
    participant MM as MountManager
    participant Fstab as /etc/fstab
    participant Shell

    MM->>Fstab: read all lines
    loop setiap entry (non-comment)
        MM->>Shell: mountpoint {dir}
        MM->>MM: status Enabled/Disabled
        MM->>Shell: mkdir -p {dir}
    end
    MM-->>User: Blade dengan list fstab
```

## Enable mount

**Route:** `GET /admin/mount/enable?device=&dir=`

```mermaid
sequenceDiagram
    actor User
    participant MM as MountManager
    participant Shell

    User->>MM: GET enable { device, dir }
    MM->>Shell: nohup mount {dir} > /tmp/{hash}.log &
    MM->>MM: sleep(3)
    MM->>Shell: read log, cek failed/no such
    MM-->>User: success / error
```

## Add / update / delete

```mermaid
sequenceDiagram
    actor User
    participant MM as MountManager
    participant Fstab

    alt add
        User->>MM: POST { device, dir, type, option, dump, fsck }
        MM->>MM: append ke list
    else update
        User->>MM: POST update (match device+dir lama)
        MM->>Shell: umount old dir
        MM->>MM: replace entry
    else delete
        User->>MM: GET delete
        MM->>Shell: umount dir
        MM->>MM: remove dari list
    end
    MM->>Fstab: save() — rewrite file
    MM-->>User: redirect + flash message
```

## Startup integration

`config/fstab_mounter.sh` dijalankan saat container start — mount semua entry fstab.

## Implikasi GoSite

```
GET    /api/v1/mounts
POST   /api/v1/mounts
PUT    /api/v1/mounts
DELETE /api/v1/mounts
POST   /api/v1/mounts/enable
```

Validasi:
- Format fstab (6 kolom)
- Device path exists atau remote valid
- Tidak allow mount sensitif tanpa konfirmasi
