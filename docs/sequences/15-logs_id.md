# Sequence: Log Viewer

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

- **Splunk Lite** — ingest + query log events ([17-splunk-lite.md](./17-splunk-lite.md), [panduan pencarian log](../guides/log-search_id.md))
- **Grafana Lite** — aggregate traffic dari access log ([18-grafana-lite.md](./18-grafana-lite.md))
- **Metrik nginx** — stub_status + VTS poll localhost ([22-nginx-metrics_id.md](./22-nginx-metrics_id.md))
- **Dashboard fallback** — `GET /system/nginx-traffic` parse access log langsung

---

## Legacy BangunSite

<details>
<summary>GET /admin/logs/get</summary>

Sama konsep path; GoSite memakai REST + auth session.

</details>

## Kode

| File | Peran |
|------|-------|
| `internal/service/logs/service.go` | Tail, list sites |
| `internal/delivery/http/handler/logs.go` | HTTP |
