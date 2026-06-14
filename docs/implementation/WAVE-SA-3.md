# WAVE SA-3 — Auth

## Scope

- `internal/service/auth/`
- `internal/delivery/http/middleware/`
- `internal/delivery/http/handler/auth.go`
- `internal/server/https.go` minimal TLS server
- Register `/health`, `/api/v1/auth/*`

## Required tests

- Login 200 + Set-Cookie; 401 bad password
- Basic auth 401 when `AUTH_ENABLE=true`
- `TestBasicAuth_DisabledBypass`
- Laravel bcrypt fixture (`$2y$`) verification
- Min 8 test functions across auth/middleware

## Forbidden

- Rate limiting code
