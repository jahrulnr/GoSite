# Sequence: SSL Management

Dua jalur: **Certbot otomatis** (job + SSE) dan **upload manual**.

## Layout filesystem SSL

```
/etc/letsencrypt  →  symlink ke /storage/webconfig/ssl
```

| Path | Isi |
|------|-----|
| `ssl/live/default/cert.pem` | Self-signed boot default |
| `ssl/live/{domain}/cert.pem` | Placeholder saat website create (Gosite) |
| `ssl/live/{domain}/fullchain.pem` | Let's Encrypt (setelah certbot) |
| `ssl/archive/{domain}/` | Archive manual / certbot |

**Konflik umum:** placeholder Gosite (`cert.pem` + `key.pem` sebagai file biasa) memblokir Certbot karena `/etc/letsencrypt/live/{domain}/` sudah ada tetapi bukan lineage LE (`CertStorageError: live directory exists`).

## A. Install SSL via Certbot (async)

**API:**

| Method | Path |
|--------|------|
| POST | `/api/v1/websites/{id}/ssl/certbot` |
| GET | `/api/v1/websites/{id}/ssl/certbot/stream?job_id=` |

```mermaid
sequenceDiagram
    actor User
    participant UI as SslModal
    participant API as SSLHandler
    participant Svc as ssl.Service
    participant Worker as job.Worker
    participant Certbot

    User->>UI: Certbot
    UI->>API: POST /ssl/certbot
    API->>Svc: EnqueueCertbot
    Svc->>Svc: prepareForCertbot
    Note over Svc: swap SSL ke default,<br/>reload, hapus placeholder
    Svc->>Svc: INSERT job_runs (pending)
    Svc->>Worker: Enqueue(job_id)
    API-->>UI: 202 { job_id }

    UI->>API: GET /ssl/certbot/stream (SSE)
    API->>Worker: StreamSSE
    Worker->>Certbot: sh -c certbot --nginx -d domain
    Certbot-->>Worker: stdout/stderr chunks
    Worker-->>API: data: ... / event: done
    API-->>UI: stream until done
```

### Command

```bash
certbot --non-interactive --agree-tos --register-unsafely-without-email --nginx -d {domain}
```

### `prepareForCertbot` (sebelum job di-queue)

Agar `nginx -t` dan Certbot tidak gagal:

1. Jika `ssl/live/{domain}/` berisi placeholder (`cert.pem` + `key.pem`, tanpa `fullchain.pem` LE):
2. Ganti `ssl_certificate` di `site.d` → path **default** self-signed
3. `UpdateSiteConfig` + `Reload`
4. Hapus direktori `ssl/live/{domain}/` placeholder
5. Baru enqueue Certbot — lineage LE bisa dibuat bersih

Tanpa langkah 2–3, menghapus placeholder saja membuat `nginx -t` gagal (cert path mengarah ke file yang tidak ada).

### Job worker

Sama dengan cron manual run (`internal/infra/job/worker.go`):

- `Enqueue(job_id)` setelah insert DB
- `StreamSSE` poll output sampai `status=ok|failed`
- Event SSE: `data:` lines + `event: done`

## B. Manual SSL upload

**API:** `PUT /api/v1/websites/{id}/ssl/manual`

```mermaid
sequenceDiagram
    actor User
    participant Svc as ssl.Service
    participant NGX as nginx.Service

    User->>Svc: { public, private } PEM
    Svc->>Svc: validatePEM
    alt site.d belum ada
        Svc->>Svc: write archive + live cert.pem/key.pem
        Svc->>NGX: WriteSiteConfig dengan ssl directives
    else site.d ada
        Svc->>Svc: overwrite path dari config
        Svc->>NGX: UpdateSiteConfig
    end
    Svc->>NGX: Reload
    Svc->>Svc: UPDATE websites.ssl = true
```

## C. Status SSL

**API:** `GET /api/v1/websites/{id}/ssl`

Membaca path dari `site.d/{domain}.conf` (`ParseCertPaths`), load PEM dari disk, hitung expiry.

## Renewal otomatis (cron default)

```
certbot renew --post-hook 'nginx -s reload'
```

Dijalankan oleh cron scheduler Go (`run_every: day`), bukan Laravel queue.

## Troubleshooting

| Gejala | Penyebab | Tindakan |
|--------|----------|----------|
| `CertStorageError: live directory exists` | Placeholder Gosite di `live/{domain}` | `prepareForCertbot` (otomatis) atau hapus manual + jalankan certbot lagi |
| Stream putus, `status=pending` | Job tidak di-enqueue ke worker | Pastikan build terbaru (fix `worker.Enqueue`) |
| `nginx -t` gagal sebelum certbot | Cert path mengarah ke file terhapus | `prepareForCertbot` swap ke default dulu |
| Certbot sukses tapi browser warning | Masih self-signed / DNS salah | Cek `ssl_certificate` path di site.d sudah ke `fullchain.pem` |

## API ringkas

| Method | Path |
|--------|------|
| GET | `/websites/{id}/ssl` |
| PUT | `/websites/{id}/ssl/manual` |
| POST | `/websites/{id}/ssl/certbot` |
| GET | `/websites/{id}/ssl/certbot/stream?job_id=` |
