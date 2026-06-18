# Sequence: Log Viewer

Tail nginx access/error log per domain or globally.

## GoSite (implementation)

**Package:** `internal/service/logs`

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
    H-->>UI: [{ domain, name }] from DB websites

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

Format `main` from `config/nginx/custom.d/nginx-log.conf`.

| domain | Access | Error |
|--------|--------|-------|
| `default` | `/storage/logs/access.log` | `/storage/logs/error.log` |
| `{domain}` | `access-{domain}.log` | `error-{domain}.log` |

### Integrasi observability

- **Splunk Lite** — ingest + query log events ([17-splunk-lite.md](./17-splunk-lite.md), [log search guide](../guides/log-search.md))
- **Grafana Lite** — aggregate traffic from access log ([18-grafana-lite.md](./18-grafana-lite.md))
- **Nginx metrics** — stub_status + VTS poll localhost ([22-nginx-metrics.md](./22-nginx-metrics.md))
- **Dashboard fallback** — `GET /system/nginx-traffic` parse access log langsung

---

## Legacy BangunSite

<details>
<summary>GET /admin/logs/get</summary>

Sama konsep path; GoSite memakai REST + auth session.

</details>

## Code

| File | Role |
|------|-------|
| `internal/service/logs/service.go` | Tail, list sites |
| `internal/delivery/http/handler/logs.go` | HTTP |
