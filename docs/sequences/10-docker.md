# Sequence: Docker Management

Mengelola container via Docker socket yang di-mount ke container BangunSite.

**Akses:** `/var/run/docker.sock` (container `privileged: true`)

## List containers

**Route:** `GET /admin/docker`

```mermaid
sequenceDiagram
    actor User
    participant DC as DockerController
    participant Cmd as Commander::shell

    User->>DC: GET /admin/docker
    DC->>Cmd: docker ps -a
    Cmd-->>DC: raw table output
    DC->>DC: parse columns (whitespace → array)
    DC-->>User: Blade table (head + rows)
```

## Restart / stop / logs

```mermaid
sequenceDiagram
    actor User
    participant DC as DockerController
    participant Cmd as Commander::shell

    User->>DC: GET /admin/docker/restart/{id}
    DC->>DC: sanitize id (alphanumeric + hyphen)
    DC->>Cmd: docker restart {id} &
    DC-->>User: raw output

    User->>DC: GET /admin/docker/stop/{id}
    DC->>Cmd: docker stop {id}
    DC-->>User: raw output

    User->>DC: GET /admin/docker/log/{id}
    DC->>Cmd: docker logs {id} -n 200
    DC-->>User: log text
```

## Keamanan legacy

- ID disanitize regex `/[^a-zA-Z0-9\-]/`
- Aksi via **GET** (seharusnya POST di GoSite)
- Tidak ada filter container — akses penuh ke semua container di host

## Implikasi GoSite

```
GET  /api/v1/docker/containers
POST /api/v1/docker/containers/{id}/restart
POST /api/v1/docker/containers/{id}/stop
GET  /api/v1/docker/containers/{id}/logs?tail=200
```

Pertimbangan:
- Gunakan Docker Engine API (HTTP via socket) daripada parse CLI
- Optional: filter container by label
- Role-based: hanya admin boleh restart/stop
