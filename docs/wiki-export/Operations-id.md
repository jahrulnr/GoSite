> **English:** [Operations](Operations)

## Sequence: Docker Management

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


## Kode

| File | Peran |
|------|-------|
| `internal/infra/docker/client.go` | Engine API wrapper |
| `internal/delivery/http/handler/docker.go` | HTTP handlers |

---

## Sequence: File Manager

Browse dan manipulasi file dalam **allowlist root**.

## GoSite (implementasi)

**Default roots:** `["/"]` (seluruh filesystem container — production harus dibatasi via config)

**Paket:** `internal/service/files` + `internal/infra/filesystem.Validator`

```mermaid
sequenceDiagram
    actor User
    participant UI as Files view
    participant H as FilesHandler
    participant Svc as files.Service
    participant FS as filesystem

    User->>UI: Browse path
    UI->>H: GET /files?path=
    H->>Svc: Browse(path)
    Svc->>Svc: Validator.Resolve — tolak ..
    Svc->>FS: readdir + stat
    H-->>UI: { entries[], tools }

    User->>UI: Edit file
    UI->>H: PUT /files/content { path, content }
    H->>Svc: Save()

    User->>UI: chmod / copy / execute
    UI->>H: POST /files/actions
    H->>Svc: Action(type, ...)
```

### API

| Method | Path | Fungsi |
|--------|------|--------|
| GET | `/files?path=` | Listing + metadata (mime, editable, archive) |
| GET | `/files/content?path=` | Baca teks |
| GET | `/files/raw?path=` | Download binary |
| PUT | `/files/content` | Simpan teks |
| POST | `/files` | Buat file/dir, upload multipart, import URL |
| POST | `/files/actions` | `chmod`, `copy`, `execute` |
| POST | `/files/batch-save` | Multi-file save |
| POST | `/files/batch-delete` | Multi-file delete |
| DELETE | `/files?path=` | Hapus file/dir |

### Actions

| type | Perilaku |
|------|----------|
| `chmod` | `chmod` via command runner |
| `copy` | Salin ke path tujuan |
| `execute` | Jalankan script — hanya jika `FILES_ALLOW_EXECUTE=true` |

### Keamanan

- `filesystem.Validator` — resolve path, tolak traversal di luar roots
- Execute dinonaktifkan default (`FILES_ALLOW_EXECUTE=false`)
- Archive extract (zip/tar) jika tool tersedia di host

### Entry metadata

Setiap entry menyertakan: `kind`, `mime_type`, `editable`, `viewable`, `archive`, `symlink`, `target`.

---


## Kode

| File | Peran |
|------|-------|
| `internal/infra/filesystem/pathutil.go` | Path validation |
| `internal/delivery/http/handler/files.go` | Multipart upload, batch ops |

---

## Sequence: Mount Manager

Kelola `/etc/fstab` (symlink → `/storage/fstab`) dan mount/umount.

## GoSite (implementasi)

**Paket:** `internal/service/mount`

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

| Path | Peran |
|------|-------|
| `/etc/fstab` | Symlink ke `/storage/fstab` |
| `/storage/mount-secrets/` | Kredensial S3 (jika entry type s3fs) |

Entry JSON dapat menyertakan blok `s3` (endpoint, bucket, keys) — disimpan terpisah dari fstab line.

### Startup

`config/start.sh` → `/run/fstab_mounter.sh` mount semua entry saat boot.

### Validasi

- Format fstab 6 kolom
- Device + dir required pada create/update
- Umount sebelum update/delete entry

---


## Kode

| File | Peran |
|------|-------|
| `internal/service/mount/service.go` | fstab CRUD, mount ops |
| `internal/delivery/http/handler/mount.go` | HTTP |

---

## Sequence: Cron Jobs

Scheduler otomatis + manual run dengan **SSE stream**.

## GoSite (implementasi)

### Scheduler (background)

**Lokasi:** `internal/app/scheduler.go` — goroutine di `gosite serve`

```mermaid
sequenceDiagram
    participant Tick as ticker 5s
    participant Sched as runCronScheduler
    participant DB as cronjobs
    participant Jobs as job_runs
    participant W as job.Worker

    loop every 5 seconds
        Tick->>Sched: now
        Sched->>DB: List cronjobs
        loop each cron
            alt ShouldRun(prev, now, run_every)
                Sched->>Jobs: Create pending job_run
                Sched->>W: Enqueue(job_id)
                Sched->>DB: TouchExecutedAt
            end
        end
    end
```

`ShouldRun` (`internal/service/cron/service.go`):

| run_every | Trigger |
|-----------|---------|
| `min` | Menit berganti |
| `hour` | Jam berganti |
| `day` | Hari berganti |
| `month` | Bulan berganti |

### Worker

Sama dengan Certbot — `internal/infra/job/worker.go`:

1. `MarkRunningWithOutput`
2. `sh -c {payload}` dengan streaming stdout/stderr
3. `Complete` status `ok` / `failed`

2 worker goroutines (buffer 32).

### CRUD

| Method | Path |
|--------|------|
| GET | `/api/v1/cronjobs` |
| POST | `/api/v1/cronjobs` |
| PUT | `/api/v1/cronjobs/{id}` |
| DELETE | `/api/v1/cronjobs/{id}` |

### Manual run + SSE

```mermaid
sequenceDiagram
    actor User
    participant UI as Cron view
    participant H as CronHandler
    participant Svc as cron.Service
    participant W as job.Worker

    User->>UI: Run now
    UI->>H: POST /cronjobs/{id}/run
    H->>Svc: RunManual
    Svc->>Svc: INSERT job_run + Enqueue
    H-->>UI: 202 { job_id }

    UI->>H: GET /cronjobs/{id}/run/stream?job_id=
    H->>W: StreamSSE
    W-->>UI: data: output ... event: done
```

Frontend: `web/src/lib/sse.ts` + `JobStreamModal`.

### Default seed

```text
certbot renew --post-hook 'nginx -s reload'
run_every: day
```

### Keamanan

Payload dijalankan sebagai shell command — pertimbangkan allowlist di produksi. Hanya user dengan session panel.

---


## Kode

| File | Peran |
|------|-------|
| `internal/app/scheduler.go` | Auto dispatch |
| `internal/infra/job/worker.go` | Exec + StreamSSE |
| `internal/delivery/http/handler/cron.go` | Run + RunStream |

---

## Sequence: Settings

GoSite hanya mengimplementasikan **update profil user**. Modul PHP/FPM legacy tidak di-port.

## GoSite (implementasi)

```mermaid
sequenceDiagram
    actor User
    participant UI as Settings view
    participant H as SettingsHandler
    participant Svc as settings.Service
    participant Auth as auth.Service
    participant DB as users

    User->>UI: Edit name, email, password
    UI->>H: PUT /settings/profile
    H->>Svc: UpdateProfile
    Svc->>DB: bcrypt password if set
    H-->>UI: { id, name, email }
```

### API

| Method | Path | Status |
|--------|------|--------|
| PUT | `/api/v1/settings/profile` | ✅ Implemented |

Profil user saat ini dibaca via `GET /auth/me`.

### Validasi

- Name & email required
- Password optional; jika diisi minimum 6 karakter
- bcrypt hash (compatible Laravel `$2y$` prefix)

---


## Kode

| File | Peran |
|------|-------|
| `internal/service/settings/service.go` | UpdateProfile |
| `internal/delivery/http/handler/settings.go` | HTTP |

UI hints: `GET /ui/meta` → section settings labels.

---

## Sequence: Log Viewer

Tail nginx access/error log per domain atau global.

## GoSite (implementasi)

**Paket:** `internal/service/logs`

```mermaid
sequenceDiagram
    actor User
    participant UI as Logs view
    participant H as LogsHandler
    participant Svc as logs.Service
    participant FS as /storage/logs

    User->>UI: Pilih site + type
    UI->>H: GET /logs/sites
    H->>Svc: ListSites()
    H-->>UI: [{ domain, name }] dari DB websites

    UI->>H: GET /logs?domain=&type=access|error&tail=
    H->>Svc: Tail(domain, type, tail)
    Svc->>FS: read last N lines
    H-->>UI: plain text
```

### API

| Method | Path | Query |
|--------|------|-------|
| GET | `/logs/sites` | — |
| GET | `/logs` | `domain`, `type` (`access`\|`error`), `tail` (default 1000) |

### Path log

Format `main` dari `config/nginx/custom.d/nginx-log.conf`.

| domain | Access | Error |
|--------|--------|-------|
| `default` | `/storage/logs/access.log` | `/storage/logs/error.log` |
| `{domain}` | `access-{domain}.log` | `error-{domain}.log` |

### Integrasi observability

- **Splunk Lite** — ingest + query log events ([17-splunk-lite.md](Observability-id))
- **Grafana Lite** — aggregate traffic dari access log ([18-grafana-lite.md](Observability-id))
- **Dashboard fallback** — `GET /system/nginx-traffic` parse access log langsung

---


## Kode

| File | Peran |
|------|-------|
| `internal/service/logs/service.go` | Tail, list sites |
| `internal/delivery/http/handler/logs.go` | HTTP |

---

## Sequence: Database Viewer

Admin tool **read-only** untuk SQLite panel.

## GoSite (implementasi)

**Paket:** `internal/service/database`

```mermaid
sequenceDiagram
    actor User
    participant UI as Database view
    participant H as DatabaseHandler
    participant Svc as database.Service
    participant DB as /storage/db.sqlite

    User->>UI: Open viewer
    UI->>H: GET /database/tables
    H->>Svc: ListTables()
    Svc->>DB: sqlite_master
    H-->>UI: ["users", "websites", "job_runs", ...]

    User->>UI: Select table
    UI->>H: GET /database/tables/{name}?limit=&offset=
    H->>Svc: GetTable(name, limit, offset)
    H-->>UI: { columns[], rows[][] }
```

### API

| Method | Path | Query |
|--------|------|-------|
| GET | `/database/tables` | — |
| GET | `/database/tables/{name}` | `limit`, `offset` |

### Batasan

- **Read-only** — tidak ada INSERT/UPDATE/DELETE dari UI
- Hanya file `db.sqlite` panel
- Session + basic auth required
- Pagination via `limit` / `offset` (default limit 100)

### Schema relevan

| Table | Isi |
|-------|-----|
| `users` | Admin panel |
| `websites` | Vhost records |
| `cronjobs` | Scheduled commands |
| `job_runs` | Certbot, cron, manual runs |
| `sessions` | Auth sessions |
| `audit_logs` | Splunk Lite audit |
| `log_events` | Ingested nginx lines |
| `traffic_metrics` | Grafana Lite buckets |
| `saved_queries` | Splunk saved searches |

Migrasi: `migrations/*.sql` via `gosite migrate`.

---


## Kode

| File | Peran |
|------|-------|
| `internal/service/database/service.go` | Query tables |
| `internal/repository/sqlite` | Schema + migrations |
