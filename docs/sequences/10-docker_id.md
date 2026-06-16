# Sequence: Docker Management

Kelola container via **Docker Engine API** (`/var/run/docker.sock`).

## GoSite (implementasi)

**Paket:** `internal/infra/docker` (official SDK) → `internal/service/docker`

```mermaid
sequenceDiagram
    actor User
    participant UI as Docker view
    participant H as DockerHandler
    participant Svc as docker.Service
    participant API as Docker Engine API

    User->>UI: Open Docker
    UI->>H: GET /docker/containers
    H->>Svc: List()
    Svc->>API: ContainerList(All=true)
    API-->>UI: JSON [{ id, name, image, status, state }]

    User->>UI: Restart
    UI->>H: POST /docker/containers/{id}/restart
    Svc->>API: ContainerRestart

    User->>UI: Logs
    UI->>H: GET /docker/containers/{id}/logs?tail=200
    Svc->>API: ContainerLogs
```

### API

| Method | Path |
|--------|------|
| GET | `/api/v1/docker/containers` |
| POST | `/api/v1/docker/containers/{id}/restart` |
| POST | `/api/v1/docker/containers/{id}/stop` |
| GET | `/api/v1/docker/containers/{id}/logs?tail=` |

### Keamanan

- Container ID disanitize (`^[a-zA-Z0-9-]+$`)
- Aksi destruktif via **POST** (bukan GET legacy)
- Session + basic auth required
- Jika socket tidak tersedia → `NoopClient` (list kosong, tidak crash)

### Fallback

`dockerinfra.NoopClient` dipakai saat `NewClient()` gagal (dev tanpa socket).

---

## Legacy BangunSite

<details>
<summary>Parse output `docker ps` CLI</summary>

- `GET /admin/docker/restart/{id}` — aksi via GET
- Parse whitespace dari stdout `docker ps -a`

</details>

## Kode

| File | Peran |
|------|-------|
| `internal/infra/docker/client.go` | Engine API wrapper |
| `internal/delivery/http/handler/docker.go` | HTTP handlers |
