# Sequence: Docker Management

Manage containers via **Docker Engine API** (`/var/run/docker.sock`).

## GoSite (implementation)

**Package:** `internal/infra/docker` (official SDK) → `internal/service/docker`

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

### Security

- Container ID is sanitized (`^[a-zA-Z0-9-]+$`)
- Destructive actions via **POST** (not legacy GET)
- Session + basic auth required
- When socket unavailable → `NoopClient` (empty list, no crash)

### Fallback

`dockerinfra.NoopClient` is used when `NewClient()` fails (dev without socket).

---

## Legacy BangunSite

<details>
<summary>Parse output `docker ps` CLI</summary>

- `GET /admin/docker/restart/{id}` — aksi via GET
- Parse whitespace from stdout `docker ps -a`

</details>

## Code

| File | Role |
|------|-------|
| `internal/infra/docker/client.go` | Engine API wrapper |
| `internal/delivery/http/handler/docker.go` | HTTP handlers |
