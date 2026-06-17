# Sequence: Metrik nginx (stub_status + VTS)

Metrik performa nginx real-time tanpa Prometheus/Grafana — collector co-located di VM/container yang sama dengan GoSite.

**Status:** ✅ Diimplementasi — `internal/observability/nginxlite`

**Spesifikasi lengkap (EN):** [22-nginx-metrics.md](./22-nginx-metrics.md)

**Tracker:** [WAVE-SA-8](../implementation/WAVE-SA-8.md)

## Ringkasan

| Lapisan | Sumber | UI |
|---------|--------|-----|
| Traffic historis | Access log → Grafana Lite | Tab **Traffic** di `/metrics` |
| Koneksi real-time | `stub_status` localhost | Tab **Nginx** + Dashboard |
| Per-domain / upstream | VTS JSON localhost | Tabel VTS di tab **Nginx** |

## stub_status (Wave 1)

- Config: `config/nginx/custom.d/stub-status.conf` (`127.0.0.1:18081`)
- Collector setiap **30 detik** → tabel `nginx_status_samples`
- API: `GET /metrics/nginx/current`, `GET /metrics/nginx/series`
- Env: `GOSITE_NGINX_STUB_STATUS_URL`

## VTS (Wave 2)

- Image production: nginx dikompilasi ulang dengan `nginx-module-vts` (`docker/nginx-vts/build.sh`)
- Config: `config/nginx/custom.d/vts.conf` + `vhost_traffic_status;` di template site
- Collector setiap **30 detik** → `nginx_vts_server_samples`, `nginx_vts_upstream_samples`
- API: `GET /metrics/nginx/vts/status|servers|upstreams`
- Env: `GOSITE_NGINX_VTS_URL` (default di image: `http://127.0.0.1:18082/status/format/json`)

## Keamanan

Endpoint status hanya di `127.0.0.1`. API panel memerlukan session. Scope plugin: `metrics:read`.

## Verifikasi

```bash
curl -s http://127.0.0.1:18081/nginx_status
curl -s http://127.0.0.1:18082/status/format/json | head
```
