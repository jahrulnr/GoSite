# Sequence: Cron Jobs

Dua jalur: **scheduler otomatis** (daemon) dan **manual run** dari UI.

## Scheduler daemon

**Proses:** `php artisan run:cronjobs` (supervisor program `crond`)

```mermaid
sequenceDiagram
    participant Cron as CronJobs command
    participant DB as cronjobs
    participant Q as queue:work
    participant Job as RunCommand

    Cron->>Q: start queue:work (background)
    loop setiap 5 detik
        Cron->>Cron: baca waktu (min, hour, day, month)
        alt menit berubah & ada cron run_every=min
            Cron->>DB: dispatchCommand(cron)
        end
        alt jam berubah & run_every=hour
            Cron->>DB: dispatchCommand(cron)
        end
        alt hari berubah & run_every=day
            Cron->>DB: dispatchCommand(cron)
        end
        alt bulan berubah & run_every=month
            Cron->>DB: dispatchCommand(cron)
        end
        Cron->>Job: dispatch RunCommand(payload)
        Job->>Job: exec shell command
    end
```

## CRUD dari UI

**Routes:** `/admin/cronjob` resource

| Aksi | Route |
|------|-------|
| List | GET index |
| Create | POST store |
| Update | PUT/PATCH update |
| Delete | DELETE destroy |

## Manual run (async polling)

**Route:** `POST /admin/cronjob/run/{id}`

```mermaid
sequenceDiagram
    actor User
    participant CJ as CronJobController
    participant Disk
    participant Queue as RunCommand
    participant Tmp as /tmp/execute-{id}.log

    User->>CJ: POST run/{id}?start=true
    CJ->>Disk: write script.sh dengan payload
    CJ->>Queue: dispatch script > log
    CJ->>DB: update executed_at
    CJ-->>User: "Waiting task run on queue"

    loop poll
        User->>CJ: POST run (start=false)
        CJ->>Tmp: read log
        CJ-->>User: output text
    end
```

## Default cron: Let's Encrypt

```sql
payload: certbot renew --post-hook 'supervisorctl restart nginx'
run_every: day
```

## Implikasi GoSite

```
GET    /api/v1/cronjobs
POST   /api/v1/cronjobs
PUT    /api/v1/cronjobs/{id}
DELETE /api/v1/cronjobs/{id}
POST   /api/v1/cronjobs/{id}/run
```

Arsitektur Go:
- **Scheduler goroutine** dengan ticker + evaluasi `run_every`
- **Worker pool** untuk eksekusi command (timeout 300s seperti legacy)
- SSE untuk output manual run & certbot

Pertimbangan keamanan: whitelist command atau require admin approval untuk payload berbahaya.
