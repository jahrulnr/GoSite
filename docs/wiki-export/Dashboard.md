> **Bahasa Indonesia:** [Dashboard-id](Dashboard-id)


Dashboard menggabungkan snapshot server, traffic, SSL expiry, dan audit feed.

## GoSite (implementation)

### Initial load — aggregated dashboard

**API:** `GET /api/v1/dashboard` (session required)

```mermaid
sequenceDiagram
    actor UI as Dashboard view
    participant H as DashboardHandler
    participant Sys as system.Service
    participant SSL as ssl.Service
    participant Splunk as splunklite.Service
    participant Graf as grafanalite.Service

    UI->>H: GET /dashboard
    H->>Sys: Info()
    H->>Graf: Summary(1h) — fallback nginx traffic
    H->>SSL: ListExpiring(30)
    H->>Splunk: RecentAudit(10)
    H-->>UI: { system, traffic_summary, ssl_expiring, recent_audit }
```

Response sections:

| Key | Sumber |
|-----|--------|
| `system` | CPU, memory, storage (`/proc`, `df`) |
| `traffic_summary` | Grafana Lite `Summary(1h)` atau fallback `system.NginxTraffic` |
| `ssl_expiring` | Cert expiry ≤ 30 hari |
| `recent_audit` | 10 audit log terakhir |

### Polling detail (opsional)

Frontend can call granular endpoints for live charts:

| Method | Path | Data |
|--------|------|------|
| GET | `/system/info` | CPU, memory, storage |
| GET | `/system/network` | `/proc/net/dev` |
| GET | `/system/disk-io` | disk I/O stats |
| GET | `/system/nginx-traffic` | Parse access log per site |

All endpoints in the **protected** group require session (+ basic auth when enabled).

### Traffic metrics (Grafana Lite)

Chart traffic memakai pre-aggregated buckets — see [18-grafana-lite.md](Observability).

Collector runs every 5 minutes in the background (`internal/app/app.go`).

---


## Code

| Paket | Role |
|-------|-------|
| `internal/delivery/http/handler/dashboard.go` | Aggregator |
| `internal/service/system` | Host metrics |
| `internal/observability/grafanalite` | Traffic buckets |
