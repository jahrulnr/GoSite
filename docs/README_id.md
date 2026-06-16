# GoSite — Dokumentasi

Dokumentasi runtime, API, dan migrasi dari **BangunSite (Laravel)** ke **GoSite (Go + Preact)**.

> **Wiki GitHub:** struktur halaman wiki disarankan di [wiki.md](./wiki_id.md).

## Status dokumen

| Kategori | Status |
|----------|--------|
| Arsitektur & domain model | ✅ Selaras implementasi |
| Semua sequence (01–19) + nginx-repair | ✅ Diperbarui ke GoSite |
| Template plugin | `plugins/_templates/` (scaffold tier 0–3) |
| `api/openapi.yaml` | Sumber kontrak API terbaru |
| `migration/` | Referensi legacy + status fase implementasi |

## Sumber kebenaran

| Item | Lokasi |
|------|--------|
| Repo | `/apps/profile/gosite` |
| API OpenAPI | `api/openapi.yaml` |
| Backend Go | `internal/` |
| Frontend | `web/` |
| Config nginx / webconfig | `config/nginx`, `config/webconfig` |
| Data produksi | Volume `/storage` |
| Legacy (referensi) | `/apps/profile/bangunsite` |

## Peta dokumen

| Dokumen | Isi |
|---------|-----|
| [architecture.md](./architecture_id.md) | Runtime container, modul Go, path persisten |
| [domain-model.md](./domain-model_id.md) | Entitas SQLite & filesystem |
| [api-inventory.md](./api-inventory_id.md) | Route legacy → REST GoSite |
| [nginx-repair.md](./nginx-repair_id.md) | Fallback `nginx -t` + auto-fix |
| [wiki.md](./wiki_id.md) | Panduan struktur wiki GitHub |
| [sequences/](./sequences/) | Diagram alur per fitur (Mermaid) |
| [migration/](./migration/) | Pembagian paket & fase migrasi |


## Modul fitur (14 area)

```
┌─────────────────────────────────────────────────────────────┐
│  Runtime & Infra                                            │
│  ├── Container startup                                      │
│  └── TLS proxy panel (:8080)                                │
├─────────────────────────────────────────────────────────────┤
│  Auth & Session                                             │
│  ├── HTTP Basic Auth (opsional)                             │
│  ├── Login / lockscreen                                     │
│  └── Rate limit login                                       │
├─────────────────────────────────────────────────────────────┤
│  Dashboard & Monitoring                                     │
│  ├── Server info (CPU, RAM, disk)                           │
│  ├── Network traffic                                        │
│  └── Nginx access traffic per site                          │
├─────────────────────────────────────────────────────────────┤
│  Website / Nginx / SSL                                        │
│  ├── CRUD website + generate vhost                          │
│  ├── Enable / disable site (symlink active.d)               │
│  ├── Edit nginx config (site, default, global)              │
│  ├── SSL: certbot install + manual upload                   │
│  └── Delete site (+ optional clean files)                   │
├─────────────────────────────────────────────────────────────┤
│  Docker                                                     │
│  ├── List containers                                        │
│  └── Restart / stop / logs                                  │
├─────────────────────────────────────────────────────────────┤
│  File Manager                                               │
│  ├── Browse, read, create, upload, import URL               │
│  └── chmod, copy, execute, delete                           │
├─────────────────────────────────────────────────────────────┤
│  Mount Manager (fstab)                                      │
├─────────────────────────────────────────────────────────────┤
│  Cron Jobs + Queue worker                                   │
├─────────────────────────────────────────────────────────────┤
│  Settings (profile, php.ini, php-fpm, pool)                 │
├─────────────────────────────────────────────────────────────┤
│  Log viewer                                                 │
├─────────────────────────────────────────────────────────────┤
│  SQLite database viewer                                     │
└─────────────────────────────────────────────────────────────┘
```

## Prinsip desain

1. **API-first** — Preact SPA di `/panel/`, kontrak di OpenAPI.
2. **Side-effect di OS** — nginx, certbot, docker, mount via `internal/infra`.
3. **Storage kompatibel** — path `/storage`, symlink `/etc/nginx`, `/etc/letsencrypt` sama seperti BangunSite.
4. **Test sebelum reload** — `nginx -t` + [auto-repair](./nginx-repair_id.md) sebelum `nginx -s reload`.
5. **Satu modul = satu sequence** di `sequences/` untuk review fitur.

## Urutan baca yang disarankan

1. [architecture.md](./architecture_id.md) — runtime GoSite
2. [domain-model.md](./domain-model_id.md) — data & file
3. [nginx-repair.md](./nginx-repair_id.md) — fallback nginx
4. [sequences/README.md](./sequences/README.md) — alur per fitur
5. [api-inventory.md](./api-inventory_id.md) + `api/openapi.yaml`
6. [wiki.md](./wiki_id.md) — jika membangun wiki GitHub

## Build Docker di jaringan ISP yang memblokir DNS publik

Docker build default memakai resolver `8.8.8.8` / `8.8.4.4` di bridge network. Di beberapa jaringan (mis. Biznet), DNS Google/Cloudflare diblokir sehingga pull image gagal:

```
lookup registry-1.docker.io on 8.8.4.4:53: i/o timeout
```

**Perbaikan di repo ini:** `make up` / `make build-docker` memakai `docker build --network=host` agar pull image memakai DNS host (mis. `203.142.82.222` dari Biznet). `compose.yml` tidak memakai `build --build` langsung karena bake Compose sering menolak entitlement `network.host`.

```bash
make build-docker   # docker build --network=host -t gosite:local .
make up             # build lalu docker compose up -d
```

Opsional (permanen, butuh restart Docker):

```json
// /etc/docker/daemon.json
{
  "dns": ["203.142.84.222", "203.142.82.222", "192.168.18.1"]
}
```

