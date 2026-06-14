# Sequence: Database Viewer

Admin tool untuk melihat isi SQLite panel — **bukan** manajemen database server.

**Routes:** `/admin/database`

## List tables

```mermaid
sequenceDiagram
    actor User
    participant DC as DatabaseController
    participant SQL as SQLite library

    User->>DC: GET /admin/database
    DC->>SQL: getTables()
    DC->>SQL: getPath() → /storage/db.sqlite
    DC-->>User: Blade table list
```

## Show table rows

```mermaid
sequenceDiagram
    actor User
    participant DC as DatabaseController
    participant SQL as SQLite library

    User->>DC: GET /admin/database/{table}?limit=100
    DC->>SQL: getCols(table)
    DC->>SQL: getRows(table, limit)
    DC-->>User: Blade grid cols × rows
```

## Batasan

- Read-only di UI (tidak ada insert/update/delete dari viewer)
- Hanya SQLite file panel, bukan MariaDB produksi

## Implikasi GoSite

```
GET /api/v1/database/tables
GET /api/v1/database/tables/{name}?limit=100&offset=0
```

Pertimbangan:
- Batasi ke admin role
- Optional: nonaktifkan di produksi (`DB_VIEWER_ENABLED=false`)
- Pagination proper (legacy hanya limit)

Untuk migrasi: schema tetap SQLite atau evaluasi embed + migration tool (goose/golang-migrate).
