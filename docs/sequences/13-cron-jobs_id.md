# Sequence: Cron Jobs

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

## Legacy BangunSite

<details>
<summary>PHP artisan run:cronjobs + Laravel queue</summary>

- Proses supervisor terpisah `crond`
- Manual run polling file `/tmp/execute-{id}.log`

</details>

## Kode

| File | Peran |
|------|-------|
| `internal/app/scheduler.go` | Auto dispatch |
| `internal/infra/job/worker.go` | Exec + StreamSSE |
| `internal/delivery/http/handler/cron.go` | Run + RunStream |
