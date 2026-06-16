> **Bahasa Indonesia:** [Development-id](Development-id)

## Mount testing in development

GoSite mount QA covers two cases:

1. **Mountable** — valid NFS export, Enable succeeds, status shows Mounted.
2. **Non-mountable** — invalid device/host, Enable fails with a clear error.

## Docker compose (recommended for TC-M01)

```bash
mkdir -p data/nfs-export
docker compose up -d
```

Inside the `gosite` container, use hostname `nfs` on the compose network.

| Field | Mountable example | Non-mountable example |
|-------|-------------------|------------------------|
| Device | `nfs:/export` | `192.0.2.99:/export` |
| Mount point | `/storage/mnt/nfs-test` | `/storage/mnt/nfs-bad` |
| Type | `nfs` | `nfs` |
| Options | `rw,nfsvers=4` | `rw,nfsvers=4` |

Flow: **Add** → row appears (Unmounted) → **Enable** → Mounted or error.

## Local API (`make dev-api`)

- **Non-mountable** testing works without NFS (Enable fails on bogus host).
- **Mountable** testing needs NFS reachable from the host process (install `nfs-common`, point device at a running NFS server) or use `docker compose up` instead.

---

## Quick start

### Prerequisites

- Go 1.26+
- Node.js 20+ and npm
- Docker & Docker Compose (for container deploy)
- OpenSSL (dev TLS cert generation)

### Local development

Two terminals — API and frontend dev server:

```bash
## Terminal 1 — Go API on https://localhost:8080
make dev-api

## Terminal 2 — Vite dev server on http://localhost:5173 (proxies /api)
make dev-fe
```

Default demo credentials (seeded when `DEMO_SEED=true`):

| Field | Value |
|-------|-------|
| Email | `admin@demo.com` |
| Password | `123456` |

`make dev-api` sets `AUTH_ENABLE=false` and uses `/tmp/gosite-qa/storage` so you can iterate without touching production paths.

### Docker (production-like)

```bash
make up    # build image (host network for DNS) + docker compose up -d
make down
```

After `make up`, the production stack listens on three ports:

| Port | URL | Purpose |
|------|-----|---------|
| `http://localhost/` | nginx default vhost | BangunSite welcome (from `/www/default`) |
| `https://localhost/panel/` | nginx → Go SPA embed | Control panel UI (Preact) |
| `http://localhost/api/...` | nginx → `:8080` | Panel REST API (proxied) |
| `https://localhost:8080/health` | Go service | Liveness probe |

Default login (seeded by `gosite init` on first boot):

| Field | Value |
|-------|-------|
| Basic auth | `admin` / `admin` |
| Panel login | `admin@demo.com` / `123456` |

> **Why `FE_EMBED=true` and `/panel/`?**
> GoSite runs behind nginx as the default vhost, so the panel SPA is mounted at `/panel/` (rewrite-stripped) while `/` keeps the legacy BangunSite welcome and any vhosts under `/storage/webconfig/active.d/` continue to serve their domains unchanged. The API is reverse-proxied from `/api/` to `:8080`, and `proxy_ssl_verify off` is acceptable because nginx and gosite share the container's loopback.

> On networks that block public DNS (e.g. some ISP resolvers), `make build-docker` uses `--network=host` so image pulls use the host resolver. See [docs/README.md](https://github.com/jahrulnr/GoSite/blob/master/docs/README.md#build-docker-di-jaringan-isp-yang-memblokir-dns-publik).

### Verifying the production stack

```bash
curl -s -o /dev/null -w "/ -> %{http_code}\n" http://localhost/
curl -s -o /dev/null -w "/panel/ -> %{http_code}\n" http://localhost/panel/
curl -s -o /dev/null -w "/api/v1/auth/login -> %{http_code}\n" http://localhost/api/v1/auth/login
curl -sk -o /dev/null -w "https :8080/health -> %{http_code}\n" https://localhost:8080/health

## API with basic auth
curl -sk -u admin:admin https://localhost/api/v1/auth/login
curl -sk -u admin:admin https://localhost/api/v1/dashboard
curl -sk -u admin:admin https://localhost/api/v1/database/tables
```

## Configuration

Environment variables (see [`internal/config/config.go`](https://github.com/jahrulnr/GoSite/blob/master/internal/config/config.go)):

| Variable | Default | Purpose |
|----------|---------|---------|
| `STORAGE_PATH` | `/storage` | Persistent data root |
| `DB_DATABASE` | `$STORAGE_PATH/db.sqlite` | SQLite database |
| `WEB_PATH` | `/www` | Website document roots |
| `LISTEN_ADDR` | `:8080` | HTTPS API listen address |
| `AUTH_ENABLE` | `true` | HTTP Basic auth on `/api/v1/*` |
| `FE_EMBED` | `false` | Serve built SPA from Go (`internal/delivery/http/frontend/dist`) |
| `DEMO_SEED` | — | Seed demo sites, logs, audit, traffic when `true` |

## CLI

```bash
gosite serve     # Start HTTPS API server
gosite init      # First-boot storage initialization
gosite migrate   # Apply SQL migrations
```

