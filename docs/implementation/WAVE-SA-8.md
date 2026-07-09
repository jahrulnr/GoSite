# WAVE SA-8 — Nginx metrics (stub_status + VTS)

**Status:** ✅ Complete (both waves)

**Sequence:** [22-nginx-metrics.md](../sequences/22-nginx-metrics.md)

## Wave 1 — stub_status ✅

### Scope

- [x] `config/nginx/custom.d/stub-status.conf`
- [x] `internal/observability/nginxlite/` (stub_status parse, collector, service)
- [x] `internal/repository/sqlite/nginx_status.go`
- [x] `migrations/009_nginx_status_samples.sql`
- [x] Handlers: `GET /metrics/nginx/current`, `GET /metrics/nginx/series`
- [x] UI: Traffic `/metrics` Nginx tab + Dashboard `nginx_status`
- [x] `docs/sequences/22-nginx-metrics.md`
- [x] `api/openapi.yaml` — `/metrics/nginx/*`, `DashboardResponse.nginx_status`

### Tests

- [x] `TestParseStubStatus`
- [x] `TestNginxLite_CollectorInsertsSample`
- [x] `TestNginxLite_SeriesRequestRate`
- [x] `TestObservability_NginxCurrent`

### Gate

- [x] `go test ./internal/observability/nginxlite/...`
- [x] stub_status reachable in container (`curl -s http://127.0.0.1:18081/nginx_status`)

## Wave 2 — VTS ✅

### Scope

- [x] Dockerfile: compile nginx + `nginx-module-vts` via `docker/nginx-vts/build.sh`
- [x] `config/nginx/custom.d/vts.conf`
- [x] `vhost_traffic_status;` in `site.conf` / `site-proxy.conf`
- [x] `migrations/010_nginx_vts_samples.sql`
- [x] `nginxlite.VTSCollector` + `GET /metrics/nginx/vts/*`
- [x] UI: VTS server/upstream tables on Nginx tab
- [x] `ENV GOSITE_NGINX_VTS_URL` in production image

### Tests

- [x] `TestParseVTSJSON`
- [x] `TestNginxLite_VTSCollectorInsertsSamples`
- [x] `TestNginxLite_VTSServiceTopRows`

### Gate

- [x] VTS JSON probe returns 200 in built image
- [x] Per-server rows visible after site traffic

### Out of scope (future)

- Prometheus exporter sidecar
- Latency percentiles from access log `request_time` in Grafana Lite
- Named `upstream {}` blocks in proxy templates for richer `upstreamZones`
