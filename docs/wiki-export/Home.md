> **Bahasa Indonesia:** [Home-id](Home-id)

**Modern hosting control panel** — Go backend, Preact SPA, and Nginx edge in one container. GoSite is the successor to [BangunSite](https://github.com/jahrulnr/bangunsite), rebuilt as a lightweight, API-first platform for managing websites, SSL, Docker, cron jobs, and observability on a single VM.

## Overview

GoSite is a single Go service that exposes a REST API and embeds (or proxies) a Preact frontend. Nginx remains the edge reverse proxy; Certbot, Docker, and filesystem operations are orchestrated through the same storage layout as that layout — so production vhosts stay compatible.

| Layer | Stack |
|-------|-------|
| Backend | Go 1.26, Gin, SQLite (`modernc.org/sqlite`) |
| Frontend | Preact 10, TypeScript, Vite 5 |
| Edge | Nginx 1.30, Certbot |
| Observability | Splunk Lite (audit + log query), Grafana Lite (traffic metrics) |

## Screenshots

### Dashboard — live server health & audit feed

![GoSite Dashboard](https://raw.githubusercontent.com/jahrulnr/GoSite/master/docs/screenshots/dashboard.png)

### Websites — CRUD, enable/disable, SSL & nginx config

![GoSite Websites](https://raw.githubusercontent.com/jahrulnr/GoSite/master/docs/screenshots/websites.png)

### Logs — Splunk-style query across access & error logs

![GoSite Logs](https://raw.githubusercontent.com/jahrulnr/GoSite/master/docs/screenshots/logs.png)

### Traffic — per-site metrics and status-code breakdown

![GoSite Traffic Metrics](https://raw.githubusercontent.com/jahrulnr/GoSite/master/docs/screenshots/metrics.png)

## Getting started

[Development](Development) — local setup, Docker, and production stack verification.

## Documentation

| Area | Pages |
|------|-------|
| Architecture | [Architecture](Architecture) · [Container-startup](Container-startup) · [Panel-routing](Panel-routing) |
| Website & SSL | [Website-create](Website-create) · [Nginx-auto-repair](Nginx-auto-repair) · [SSL-and-Certbot](SSL-and-Certbot) |
| Operations | [Operations](Operations) · [Observability](Observability) · [Dashboard](Dashboard) |
| Reference | [API-reference](API-reference) · [Sequences-index](Sequences-index) · [Migration](Migration) |
