# Sequence: Dashboard & Monitoring

Dashboard menampilkan ringkasan server. Data di-refresh via AJAX ke API internal.

## Initial page load

**Route:** `GET /admin/` → `HomeController@index`

```mermaid
sequenceDiagram
    actor User
    participant Home as HomeController
    participant DB as websites table
    participant Lib as Cpu/Memory/Disk/Network/Log

    User->>Home: GET /admin/
    Home->>DB: Website::count()
    Home->>Lib: Cpu::count()
    Home->>Lib: Memory::info()
    Home->>Lib: Disk::bytesReadable(disk_total_space)
    Home->>Lib: Network::traffic()
    Home->>Lib: Log::accessTraffic()
    Home-->>User: Blade Home/index (nilai awal)
```

## Polling API (client-side)

**Routes:** `routes/api.php` — saat ini **tanpa auth middleware**

```mermaid
sequenceDiagram
    actor Browser
    participant API as ServerController
    participant OS as /proc, df, free
    participant NGX as nginx access logs

    loop setiap interval (JS)
        Browser->>API: POST /api/server/info
        API->>OS: free, sys_getloadavg, df overlay
        API-->>Browser: { cpu, memory, storage }

        Browser->>API: POST /api/server/traffic
        API-->>Browser: Network::traffic()

        Browser->>API: POST /api/server/diskIO
        API-->>Browser: Disk::simpleStat()

        Browser->>API: POST /api/server/nginx/traffic
        API->>NGX: Log::accessTraffic()
        API-->>Browser: { sites[], total }
    end
```

## Data yang dikembalikan

### `/api/server/info`

- `cpu` — load average / core count × 100
- `memory[]` — label, total, used, free (dari `free`)
- `storage` — overlay filesystem dari `df`

### `/api/server/nginx/traffic`

- Per-site request count & bytes dari parse access log
- Total agregat

## Implikasi GoSite

| Endpoint Go | Sumber data |
|-------------|-------------|
| `GET /system/info` | `/proc/loadavg`, `/proc/meminfo`, `df` |
| `GET /system/network` | `/proc/net/dev` |
| `GET /system/disk-io` | iostat atau `/proc/diskstats` |
| `GET /system/nginx-traffic` | parser access log yang sama |

**Penting:** wajib auth di GoSite (legacy endpoint terbuka).

Frontend framework-agnostic: cukup `fetch` interval atau WebSocket push.
