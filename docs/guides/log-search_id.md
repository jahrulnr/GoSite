# Sintaks pencarian log

Pencarian bergaya Splunk untuk view **Logs** GoSite (`GET /api/v1/query`). Parser lama `field:value` sudah diganti — query tersimpan dengan sintaks lama perlu ditulis ulang.

## Referensi cepat

| Fitur | Contoh |
|-------|--------|
| AND implisit | `404 GET` |
| AND / OR / NOT | `error OR timeout`, `login NOT failed` |
| Frasa kutip | `"connection refused"` |
| Field | `status=404`, `action=login` (`:` alias `=`) |
| Wildcard | `status=3*`, `action=website.*` |
| Regex | `/timeout/`, `status=/^3\d{2}$/` |
| Perbandingan | `status>=300 status<400` |
| Grup | `(status=301 OR status=302)` |
| Pipe (history saja) | `\| head 50`, `\| sort -ts` |

Contoh kanonik di log **access**:

```spl
status>=399 AND (curl OR status=200)
```

Artinya: status HTTP ≥ 399 **dan** (baris mengandung `curl` **atau** status 200).

## Sumber dan field

Format access nginx: `log_format main` di [`config/nginx/custom.d/nginx-log.conf`](../../config/nginx/custom.d/nginx-log.conf).

| Sumber | Penyimpanan | Kolom ter-index | Catatan |
|--------|-------------|-----------------|---------|
| **access** | `log_events` | `status_code`, `bytes`, `site`, `ts` | Method, URI, UA di `raw_preview` |
| **error** | `log_events` | `site`, `ts` | Teks error nginx di `raw_preview` |
| **audit** | `audit_logs` | `user`, `action`, `domain`, `status`, `message`, … | Aksi panel |
| **job** | `job_runs` | `job_type`, `name`, `status`, `output`, `error` | Job latar belakang |
| **all_sources** | gabungan | per-sumber | Sumber tanpa field relevan di-skip (0 hit) |

### Access (`structured`)

- `status` / `status_code` — status HTTP
- `site` — domain vhost
- `message` / `preview` — baris penuh
- Teks bebas (`GET`, `curl`) mencari `site` dan `raw_preview`

### Error (`text`)

- Utama teks bebas: `error`, `timeout`, `/upstream/`
- Tidak ada `status_code` terstruktur

### Audit / job (`structured`)

- `status` = hasil operasi (`ok` / `failed`), bukan HTTP 404/500
- `status>=399` di audit/job saja → 0 hit

## Pipe

Hanya untuk pencarian **History**. Live Tail mengabaikan pipe.

## Breaking change

- Sintaks `field:value` **tidak** didukung lagi
- Query tersimpan mungkin gagal validasi sampai diperbarui
- Bantuan UI dari `GET /api/v1/query/meta`

Lihat [17-splunk-lite](../sequences/17-splunk-lite_id.md) untuk arsitektur.
