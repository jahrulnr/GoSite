# Sequence: Log Viewer

Membaca log nginx access/error per domain atau global.

**Routes:** `GET /admin/logs`, `GET /admin/logs/get`

## List sites untuk filter

```mermaid
sequenceDiagram
    actor User
    participant LC as LogController
    participant DB as websites

    User->>LC: GET /admin/logs
    LC->>DB: Website::orderBy(name)
    LC-->>User: Blade dropdown domain + type
```

## Fetch log content

```mermaid
sequenceDiagram
    actor User
    participant LC as LogController
    participant Log as Log library

    User->>LC: GET /admin/logs/get?domain=&type=accesslog|errorlog
    LC->>LC: map type → access | error
    alt domain != default
        LC->>Log: readLog("{type}-{domain}.log", 1000)
    else default
        LC->>Log: readLog("{type}.log", 1000)
    end
    LC-->>User: plain text log (last 1000 lines)
```

## Path log

| Log | Path |
|-----|------|
| Global access | `/storage/laravel/logs/access.log` |
| Global error | `/storage/laravel/logs/error.log` |
| Per domain access | `access-{domain}.log` |
| Per domain error | `error-{domain}.log` |

## Traffic parsing (dashboard)

`Log::accessTraffic()` — parse access log untuk statistik per site (dipakai API dashboard).

## Implikasi GoSite

```
GET /api/v1/logs/sites
GET /api/v1/logs?domain=default&type=access&tail=1000
```

Opsional:
- `GET /api/v1/logs/tail` — SSE follow mode
- Filter level, search regex

Frontend hanya render text/monospace — tidak ada preferensi framework.
