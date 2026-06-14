# WAVE SA-6 — Observability Lite

## Scope

- `internal/observability/splunklite/`
- `internal/observability/grafanalite/`
- Audit write hook using `contracts.AuditWriter`
- `docs/sequences/17-splunk-lite.md`, `18-grafana-lite.md`

## Required tests

- `TestQueryParser_Wildcard`
- `TestQuery_TimeRangeInverted`
- `TestGrafana_Rollup5mBucket`
- `TestGrafana_OffsetResumeAfterRestart`
- `TestGrafana_RetentionPurge`
- `TestAudit_WriteOnMutation`

## Gate

- Min 20 tests across splunklite + grafanalite
