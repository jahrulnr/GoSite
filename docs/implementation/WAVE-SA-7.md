# WAVE SA-7 — System, settings, logs, database

## Scope

- `internal/service/system/`, `settings/`, `logs/`, `database/`
- `internal/delivery/http/handler/dashboard.go`
- Routes: `/dashboard`, `/system/*`, `/settings/profile`, `/logs`, `/database/tables/*`

## Required tests

- Min **8** tests per package
- `TestDashboard_ContainsAllSections` — keys: `system`, `traffic_summary`, `ssl_expiring`, `recent_audit`

## Gate

- Dashboard aggregate calls SA-6 interfaces (real or injected)
