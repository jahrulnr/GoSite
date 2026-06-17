# Mount testing in development

QA mount manager GoSite — [sequences/12-mount-manager_id.md](./sequences/12-mount-manager_id.md). API: `/api/v1/mounts`. Panel: **Mounts**.

EN lengkap: [dev-mount-testing.md](./dev-mount-testing.md).

## Dua skenario

1. **Mountable** — NFS valid, Enable sukses, status Mounted.
2. **Non-mountable** — host/device invalid, Enable gagal dengan error jelas.

## Docker compose (disarankan)

```bash
mkdir -p data/nfs-export
docker compose up -d
```

Di dalam container `gosite`, hostname `nfs` di jaringan compose.

| Field | Contoh mountable | Contoh gagal |
|-------|------------------|--------------|
| Device | `nfs:/export` | `192.0.2.99:/export` |
| Mount point | `/storage/mnt/nfs-test` | `/storage/mnt/nfs-bad` |
| Type | `nfs` | `nfs` |
| Options | `rw,nfsvers=4` | `rw,nfsvers=4` |

Alur: **Add** → **Enable** → Mounted atau error.

## `make dev-api`

- Uji **non-mountable** tanpa NFS.
- Uji **mountable** butuh NFS dari host atau pakai `docker compose up`.
