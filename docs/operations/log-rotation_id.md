# Rotasi Log & Maintenance SQLite

**Status:** Implemented

GoSite berjalan dalam satu container Docker dengan nginx dan `gosite serve` sebagai proses paralel. Tanpa rotasi log aktif, file `.log` nginx di `/storage/logs/` tumbuh tanpa batas. SQLite (`/storage/db.sqlite`) juga bisa bengkak setelah retention purge meninggalkan free pages.

## Rotasi log (nginx raw logs)

### Konfigurasi

| File | Peran |
|------|------|
| `config/logrotate/gosite` | Konfigurasi logrotate (di-copy ke `/etc/logrotate.d/gosite` di image) |
| `Dockerfile` | Install paket `logrotate` |
| `config/start.sh` | Background loop: `logrotate /etc/logrotate.d/gosite` setiap 24 jam |

### Kebijakan

```
/storage/logs/*.log {
    daily
    rotate 14
    maxage 14
    compress
    delaycompress
    missingok
    notifempty
    copytruncate
    dateext
    dateformat -%Y%m%d
}
```

- Rotasi **harian**
- **14 rotasi** disimpan (maksimal 2 minggu)
- **compress** + **delaycompress** (gzip rotasi sebelumnya, bukan yang aktif)
- **copytruncate** (tidak perlu reload nginx — copy lalu truncate file aktif)
- **dateext** — file rotasi bernama `access.log-20260711.gz` dst.

### Cara kerja

`start.sh` menjalankan background subshell sebelum `exec gosite serve`:

```bash
(
  sleep 60   # tunggu nginx buka file log
  while true; do
    logrotate /etc/logrotate.d/gosite 2>/dev/null || true
    sleep 86400
  done
) &
```

Tidak butuh cron daemon — loop self-contained.

## Maintenance SQLite (VACUUM)

### Masalah

Loop retention purge (`internal/app/app.go` → `runRetentionPurge`) menghapus baris kadaluarsa dari `audit_logs`, `log_events`, `traffic_metrics`, `nginx_status_samples`, dan `nginx_vts_*_samples` setiap 24 jam. SQLite menandai page yang dihapus sebagai free, tetapi **tidak** mengecilkan file database — page free dipakai ulang untuk insert berikutnya, tapi ukuran file tidak pernah berkurang.

### Solusi

Setelah semua retention purge selesai, `runRetentionPurge` memanggil `sqlite.Vacuum(db)` untuk menjalankan `VACUUM` yang membangun ulang file database dan mengklaim kembali ruang free.

| File | Peran |
|------|------|
| `internal/repository/sqlite/db.go` → `Vacuum()` | Eksekusi `VACUUM` pada database |
| `internal/app/app.go` → `runRetentionPurge` | Memanggil `Vacuum` setelah semua purge |

### Default retention

| Tabel | Env var | Default |
|-------|---------|---------|
| `audit_logs` | `AUDIT_RETENTION_DAYS` | 90 hari |
| `log_events` | `LOG_EVENTS_RETENTION_DAYS` | 14 hari |
| `traffic_metrics` | `LOG_EVENTS_RETENTION_DAYS` | 14 hari |
| `nginx_status_samples` | `LOG_EVENTS_RETENTION_DAYS` | 14 hari |
| `nginx_vts_*_samples` | `LOG_EVENTS_RETENTION_DAYS` | 14 hari |

### Jadwal

Rotasi log dan retention purge + VACUUM keduanya berjalan setiap **24 jam**. VACUUM pertama berjalan 24 jam setelah container start.

## Code

| File | Peran |
|------|------|
| `config/logrotate/gosite` | Kebijakan logrotate |
| `config/start.sh` | Background logrotate loop |
| `Dockerfile` | Install `logrotate`, copy config |
| `internal/repository/sqlite/db.go` | Fungsi `Vacuum()` |
| `internal/app/app.go` | `runRetentionPurge` → purge + VACUUM |
