# Sequence: Database Viewer

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

## Legacy BangunSite

<details>
<summary>Blade grid viewer</summary>

Sama read-only; GoSite menambah offset pagination.

</details>

## Kode

| File | Peran |
|------|-------|
| `internal/service/database/service.go` | Query tables |
| `internal/repository/sqlite` | Schema + migrations |
