# Sequence: Mount Manager

Manage `/etc/fstab` (symlink → `/storage/fstab`) dan mount/umount.

## GoSite (implementation)

**Package:** `internal/service/mount`

```mermaid
sequenceDiagram
    actor User
    participant H as MountHandler
    participant Svc as mount.Service
    participant Fstab as /etc/fstab
    participant Cmd as commander

    User->>H: GET /mounts
    H->>Svc: List()
    Svc->>Fstab: parse lines
    loop each entry
        Svc->>Cmd: mountpoint {dir}
    end
    H-->>User: [{ device, dir, type, mounted, s3? }]

    User->>H: POST /mounts/enable
    H->>Svc: Enable(device, dir)
    Svc->>Cmd: mount {dir}
```

### API

| Method | Path |
|--------|------|
| GET | `/api/v1/mounts` |
| POST | `/api/v1/mounts` |
| PUT | `/api/v1/mounts` |
| DELETE | `/api/v1/mounts` |
| POST | `/api/v1/mounts/enable` |

### fstab & secrets

| Path | Role |
|------|-------|
| `/etc/fstab` | Symlink ke `/storage/fstab` |
| `/storage/mount-secrets/` | S3 credentials (for s3fs entry type) |

Entry JSON may include an `s3` block (endpoint, bucket, keys) — stored separately from the fstab line.

### Startup

`config/start.sh` → `/run/fstab_mounter.sh` mount all entries at boot.

### Validation

- Format fstab 6 kolom
- Device + dir required on create/update
- Umount before update/delete entry

---

## Legacy BangunSite

<details>
<summary>GET for enable/delete</summary>

- Enable via `GET /admin/mount/enable?device=&dir=`
- Async mount log di `/tmp/{hash}.log`

GoSite memakai POST dan response JSON langsung.

</details>

## Code

| File | Role |
|------|-------|
| `internal/service/mount/service.go` | fstab CRUD, mount ops |
| `internal/delivery/http/handler/mount.go` | HTTP |
