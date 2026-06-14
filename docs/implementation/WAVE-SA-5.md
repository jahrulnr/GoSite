# WAVE SA-5 — Ops modules

## Scope

- `internal/service/docker/`, `files/`, `mount/`, `cron/`
- `internal/infra/job/` (SSE worker pool)

## Required edge tests

- `TestFiles_PathTraversalRejected`
- `TestFiles_ExecuteDisabledByDefault`
- `TestCron_MonthRollover`
- `TestJob_SSEStreamsOutput`

## Gate

- Min **10** tests per package (docker, files, mount, cron, job)
- All docker mutations via `POST`
