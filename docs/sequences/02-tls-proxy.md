# Sequence: Panel Routing & TLS

GoSite **does not** use a separate `server-proxy` binary like legacy BangunSite. The panel is served by **nginx edge** + **gosite serve**.

## GoSite (implementation)

```mermaid
flowchart LR
    subgraph Internet
        P443[":443 / :80"]
    end

    subgraph Container
        NGX[Nginx edge]
        APP["gosite serve :8080"]
        VHOST["active.d vhosts"]
    end

    P443 --> NGX
    NGX -->|"/panel/*"| APP
    NGX -->|"/api/*"| APP
    NGX -->|server_name domain| VHOST
    NGX -->|"/ default vhost"| WWW["/www/default"]
```

| Path publik | Handler | Notes |
|-------------|---------|---------|
| `/` | nginx default vhost | Welcome page `/www/default` |
| `/panel/` | nginx → gosite | SPA Preact (embed when `FE_EMBED=true`) |
| `/api/v1/*` | nginx → `http://127.0.0.1:8080` | REST API |
| `https://:8080/health` | gosite langsung | Health check TLS (loopback) |

`gosite serve` listens on `LISTEN_ADDR` (default `:8080`) with optional TLS (`TLS_ENABLE`).

### Edge middleware

- **HTTP Basic Auth** (`AUTH_ENABLE`) — gate `/api/v1/*` before session
- **Session cookie** — after `POST /auth/login`
- Nginx `proxy_ssl_verify off` for upstream loopback (container shared network)

### Relevant configuration

| Env | Default | Role |
|-----|---------|-------|
| `LISTEN_ADDR` | `:8080` | Bind API + SPA |
| `TLS_ENABLE` | `true` | HTTPS on gosite |
| `FE_EMBED` | `false` | Serve `frontend/dist` from binary |
| `AUTH_ENABLE` | `true` | Basic auth layer |

Dev/prod URL details: [README.md](../README.md#verifying-the-production-stack).

---

## Legacy BangunSite

<details>
<summary>Go TLS proxy :8080 → Laravel :8000</summary>

Binary `proxy/main.go` accepted HTTPS on `:8080`, forwarding to `http://localhost:8000` (PHP artisan).

| Item | Value |
|------|-------|
| Listen | `:8080` |
| Upstream | Laravel :8000 |
| Cert | `/storage/webconfig/ssl/live/default/` |

Replaced by nginx reverse-proxy to `gosite serve` + TLS directly in Go.

</details>
