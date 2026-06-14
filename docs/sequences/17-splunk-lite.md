# Sequence: Splunk Lite

Query internal audit, job, and nginx log events without deploying Splunk.

**Routes:** `GET /api/v1/query/meta`, `POST /api/v1/query`, `GET/POST /api/v1/query/saved`

## Write audit on mutation

```mermaid
sequenceDiagram
    actor User
    participant API as WebsiteHandler
    participant AW as AuditWriter
    participant DB as audit_logs

    User->>API: POST /websites (mutating request)
    API->>API: execute business logic
    API->>AW: Write(user, action, status, meta)
    AW->>DB: INSERT audit_logs
    API-->>User: 201 Created
```

## Backend-driven query metadata

```mermaid
sequenceDiagram
    actor User
    participant H as ObservabilityHandler
    participant M as splunklite.MetaService
    participant L as logs.Service
    participant FS as Nginx log directory

    User->>H: GET /query/meta
    H->>M: Meta()
    M->>L: ListSites()
    M->>FS: scan access-*.log/error-*.log
    M-->>H: sources[], fields[], quick_filters[], examples[]
    H-->>User: JSON metadata
```

The frontend renders query sources only from this response. Per-vhost nginx entries use IDs such as `access:example.com` and carry the backend payload `{ "source": "access", "site": "example.com" }`.

## Search events

```mermaid
sequenceDiagram
    actor User
    participant H as ObservabilityHandler
    participant I as splunklite.LogIngestor
    participant S as splunklite.Service
    participant DB as SQLite sources

    User->>H: POST /query { source, site, q, from, to, limit }
    H->>I: Ingest nginx log files
    I->>DB: INSERT OR IGNORE log_events by line_hash
    H->>S: Query(request)
    S->>S: ParseQuery(q) field:value + wildcard *
    alt source=audit
        S->>DB: SELECT audit_logs
    else source=job
        S->>DB: SELECT job_runs
    else source=access|error
        S->>DB: SELECT log_events
    else source=all
        S->>DB: merge audit + job + logs
    end
    S-->>H: { hits, events[] }
    H-->>User: JSON
```

## Mini query syntax

| Token | Meaning |
|-------|---------|
| `field:value` | Exact match |
| `field:prefix*` | Wildcard (`*` → SQL LIKE) |
| space | AND (implicit) |

**Audit fields:** `user`, `action`, `resource_type`, `resource_id`, `domain`, `status`, `message`

**Job fields:** `type`, `name`, `status`, `output`, `error`

**Log fields:** `site`, `status`, `status_code`, `message`, `preview`

## Example

```json
POST /api/v1/query
{
  "source": "audit",
  "q": "action:website.* user:admin@* status:ok",
  "from": "2026-06-01T00:00:00Z",
  "to": "2026-06-14T23:59:59Z",
  "limit": 50
}
```

## Retention

| Table | Default retention |
|-------|-------------------|
| `audit_logs` | 90 days (`AUDIT_RETENTION_DAYS`) |
| `log_events` | 14 days (`LOG_EVENTS_RETENTION_DAYS`) |

## Implikasi GoSite

- `internal/observability/splunklite` — parser + query service
- `contracts.AuditWriter` — hook untuk semua mutasi sensitif
- Saved queries di `saved_queries` untuk preset dashboard / ops
