# GoSite

**Modern hosting control panel** — Go backend, Preact SPA, and Nginx edge in one container. GoSite is the successor to [BangunSite](https://github.com/jahrulnr/bangunsite) (Laravel), rebuilt as a lightweight, API-first platform for managing websites, SSL, Docker, cron jobs, and observability on a single VM.

[![Go](https://img.shields.io/badge/Go-1.26.4-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

---

## Overview

GoSite replaces a multi-process Laravel stack with a single Go service that exposes a REST API and embeds (or proxies) a Preact frontend. Nginx remains the edge reverse proxy; Certbot, Docker, and filesystem operations are orchestrated through the same storage layout as the legacy panel — so production vhosts stay compatible.

| Layer | Stack |
|-------|-------|
| Backend | Go 1.26, Gin, SQLite (`modernc.org/sqlite`) |
| Frontend | Preact 10, TypeScript, Vite 5 |
| Edge | Nginx 1.30, Certbot, Supervisor |
| Observability | Splunk Lite (audit + log query), Grafana Lite (traffic metrics) |

## Screenshots

### Dashboard — live server health & audit feed

![GoSite Dashboard](docs/screenshots/dashboard.png)

### Websites — CRUD, enable/disable, SSL & nginx config

![GoSite Websites](docs/screenshots/websites.png)

### Logs — Splunk-style query across access & error logs

![GoSite Logs](docs/screenshots/logs.png)

### Traffic — per-site metrics and status-code breakdown

![GoSite Traffic Metrics](docs/screenshots/metrics.png)

## Features

- **Dashboard** — CPU, RAM, disk, network I/O, SSL expiry watch, top sites, recent audit
- **Websites** — static & reverse-proxy vhosts, enable/disable via `active.d/` symlinks
- **Nginx & SSL** — edit global/default config, `nginx -t`, reload, Certbot or manual certs
- **Docker** — list, restart, stop containers; stream logs via `docker.sock`
- **File manager** — browse `/www` and storage roots with path validation
- **Mount manager** — fstab CRUD and mount/umount
- **Cron jobs** — schedule + manual run with SSE stream
- **Splunk Lite** — structured log ingest, saved queries, tail stream
- **Grafana Lite** — traffic time-series, top sites, status-code charts
- **Database viewer** — read-only SQLite table browser
- **Auth** — session cookies, optional HTTP Basic gate, lockscreen

## Quick start

### Prerequisites

- Go 1.26+
- Node.js 20+ and npm
- Docker & Docker Compose (for container deploy)
- OpenSSL (dev TLS cert generation)

### Local development

Two terminals — API and frontend dev server:

```bash
# Terminal 1 — Go API on https://localhost:8080
make dev-api

# Terminal 2 — Vite dev server on http://localhost:5173 (proxies /api)
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

Panel API: `https://localhost:8080/api/v1` · Health: `https://localhost:8080/health`

> On networks that block public DNS (e.g. some ISP resolvers), `make build-docker` uses `--network=host` so image pulls use the host resolver. See [docs/README.md](docs/README.md#build-docker-di-jaringan-isp-yang-memblokir-dns-publik).

## Configuration

Environment variables (see [`internal/config/config.go`](internal/config/config.go)):

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

## API

OpenAPI 3.1 spec: [`api/openapi.yaml`](api/openapi.yaml)

```bash
make contract-check   # Golden JSON contract tests
```

Base URL: `https://<host>:8080/api/v1` — session cookie `gosite_session` after `POST /auth/login`.

## Development

```bash
make build          # Build frontend + Go binary → bin/gosite
make test           # go test -race
make test-cover     # Service/observability packages ≥80% coverage gate
make contract-check # API response shape tests
```

### Project layout

```
cmd/gosite/          CLI entrypoint
internal/
  delivery/http/     Gin handlers, middleware, embedded frontend
  service/           Business logic (auth, website, docker, …)
  repository/sqlite/ Data access
  observability/     Splunk Lite + Grafana Lite
  infra/             nginx, docker, commander, job worker
web/                 Preact SPA (Vite → dist embed)
api/                 OpenAPI spec + golden examples
config/              nginx templates, supervisord, bootstrap scripts
migrations/          SQLite schema
docs/                Architecture, sequences, migration guides
```

## Architecture

GoSite runs inside a single container: **Nginx** (80/443), **Go panel** (8080 HTTPS), **Supervisor**, and **Certbot**. Storage paths mirror BangunSite for drop-in migration.

```mermaid
flowchart LR
    subgraph Client
        Browser["Browser / API client"]
    end

    subgraph Container["gosite container"]
        NGX["Nginx :80/:443"]
        API["Go API :8080"]
        DB[("SQLite")]
        SOCK["docker.sock"]
        STG["/storage volume"]
    end

    Browser --> NGX
    Browser --> API
    API --> DB
    API --> STG
    API --> SOCK
    NGX --> STG
```

Deep dive: [docs/architecture.md](docs/architecture.md) · Feature sequences: [docs/sequences/](docs/sequences/)

## Migration from BangunSite

GoSite preserves `/storage`, `/www`, and nginx vhost layout. The docs tree maps every legacy Laravel route to the new REST API:

1. [docs/architecture.md](docs/architecture.md) — runtime & module boundaries
2. [docs/domain-model.md](docs/domain-model.md) — entities & filesystem
3. [docs/api-inventory.md](docs/api-inventory.md) — Laravel → Go endpoint map
4. [docs/migration/backend-modules.md](docs/migration/backend-modules.md) — implementation waves

## License

MIT — see [LICENSE](LICENSE) if present, or add your preferred license.

## Author

[jahrulnr](https://github.com/jahrulnr) — BangunSoft / GoSite
