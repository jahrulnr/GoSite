# GoSite — Dokumentasi Migrasi

Dokumentasi ini mendeskripsikan **BangunSite (Laravel)** sebagai acuan untuk migrasi ke **GoSite (Go backend + frontend terpisah)**.

Tujuan dokumen ini: memetakan perilaku sistem *sebelum* memilih framework frontend (Vue, React, Angular, dll.). Frontend nanti hanya perlu mengonsumsi kontrak API yang sama.

## Sumber kebenaran (legacy)


| Item        | Lokasi                                          |
| ----------- | ----------------------------------------------- |
| Repo legacy | `/apps/profile/bangunsite`                      |
| Panel admin | Laravel 10, PHP 8.2, AdminLTE (server-rendered) |
| Edge proxy  | Nginx + Go TLS proxy (`proxy/main.go`)          |
| Data        | SQLite di `/storage/db.sqlite`                  |
| Persistensi | Volume `./data` → `/storage`                    |


## Peta dokumen


| Dokumen                                                            | Isi                                             |
| ------------------------------------------------------------------ | ----------------------------------------------- |
| [architecture.md](./architecture.md)                               | Arsitektur runtime, modul, batas tanggung jawab |
| [domain-model.md](./domain-model.md)                               | Entitas data & file system                      |
| [api-inventory.md](./api-inventory.md)                             | Route Laravel saat ini → usulan REST API Go     |
| [sequences/](./sequences/)                                         | Sequence diagram per fitur (Mermaid)            |
| [migration/backend-modules.md](./migration/backend-modules.md)     | Pembagian paket Go & urutan implementasi        |


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

## Prinsip migrasi

1. **Backend Go** menggantikan Laravel; tidak ada Blade/server-rendered.
2. **Kontrak API dulu** — frontend hanya consumer JSON; pilih framework belakangan.
3. **Side-effect tetap di OS** — nginx reload, certbot, docker CLI, mount, shell; Go memanggil proses yang sama.
4. **Storage path tidak berubah** — `/storage`, `/www`, `/etc/nginx` symlink tetap kompatibel dengan deploy produksi.
5. **Satu modul = satu sequence** di `sequences/` untuk review & implementasi bertahap.

## Urutan baca yang disarankan

1. [architecture.md](./architecture.md) — pahami runtime
2. [domain-model.md](./domain-model.md) — pahami data
3. [sequences/README.md](./sequences/README.md) — telusuri alur per fitur
4. [api-inventory.md](./api-inventory.md) — desain endpoint Go
5. [migration/backend-modules.md](./migration/backend-modules.md) — rencana implementasi

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

