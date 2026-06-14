# WAVE SA-1 — Foundation

## Scope

- `go.mod`, `Makefile`
- `pkg/apperror/`
- `internal/contracts/`
- `internal/config/`
- `migrations/*.sql`
- `internal/repository/sqlite/`
- `internal/testutil/`
- `cmd/gosite/main.go` (stub subcommands)

## Deliverables

| Item | Done when |
|------|-----------|
| Dependencies | gin v1.10, modernc.org/sqlite, testify, golang.org/x/crypto |
| Migrations | `001_legacy.sql` + `002_gosite_extensions.sql` |
| Repository | `Open()`, `Migrate()`, user `Create`/`Find` |
| Makefile | `build`, `test`, `test-cover` targets |

## Required tests

- `TestConfig_LoadDefaults`
- `TestMigrate_ApplyAllTables`
- `TestMigrate_Idempotent`
- `TestUserRepo_CreateFind`

## Forbidden

- HTTP handlers
- nginx / docker / ssl logic
- bootstrap / Dockerfile

## Handoff

Return file list + `go test -race` output for created packages.
