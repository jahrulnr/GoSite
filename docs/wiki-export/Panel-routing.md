> **Bahasa Indonesia:** [Panel-routing-id](Panel-routing-id)


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

Dev/prod URL details: [README.md](Development#verifying-the-production-stack).

---

