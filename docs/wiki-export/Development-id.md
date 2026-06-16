> **English:** [Development](Development)

## Mount testing in development

GoSite mount QA covers two cases:

1. **Mountable** — valid NFS export, Enable succeeds, status shows Mounted.
2. **Non-mountable** — invalid device/host, Enable fails with a clear error.

## Docker compose (recommended for TC-M01)

```bash
mkdir -p data/nfs-export
docker compose up -d
```

Inside the `gosite` container, use hostname `nfs` on the compose network.

| Field | Mountable example | Non-mountable example |
|-------|-------------------|------------------------|
| Device | `nfs:/export` | `192.0.2.99:/export` |
| Mount point | `/storage/mnt/nfs-test` | `/storage/mnt/nfs-bad` |
| Type | `nfs` | `nfs` |
| Options | `rw,nfsvers=4` | `rw,nfsvers=4` |

Flow: **Add** → row appears (Unmounted) → **Enable** → Mounted or error.

## Local API (`make dev-api`)

- **Non-mountable** testing works without NFS (Enable fails on bogus host).
- **Mountable** testing needs NFS reachable from the host process (install `nfs-common`, point device at a running NFS server) or use `docker compose up` instead.

---

## Quick start

### Prerequisites

- Go 1.26+
(Detail lengkap: lihat README repo dan halaman Development-id.)
