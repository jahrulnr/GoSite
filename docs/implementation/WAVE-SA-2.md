# WAVE SA-2 — Runtime + Docker image

## Scope

- `internal/bootstrap/init.go`
- `config/**` templates (nginx, webconfig, supervisord, start.sh)
- `config/webconfig/site-proxy.conf`
- `Dockerfile`, `compose.yml`
- Wire `gosite init` and `gosite migrate` subcommands

## Required tests

- `TestBootstrap_CreatesStorageLayout`
- `TestBootstrap_IdempotentSecondRun`
- `TestBootstrap_Symlinks`
- `TestBootstrap_SeedsAdminWhenEmpty`

## Gate

- `make build` produces image/binary
- `gosite init` idempotent per seq-01
