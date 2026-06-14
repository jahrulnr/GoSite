# Sequence: Authentication

Tiga lapisan autentikasi di BangunSite.

## 1. HTTP Basic Auth (opsional)

**Middleware:** `BasicAuth` — aktif jika `AUTH_ENABLE=true`

```mermaid
sequenceDiagram
    actor Browser
    participant MW as BasicAuth Middleware
    participant App as Controller

    Browser->>MW: Request /
    alt AUTH_ENABLE=false
        MW->>App: next()
    else AUTH_ENABLE=true
        alt kredensial valid
            MW->>App: next()
        else tidak valid
            MW-->>Browser: 401 WWW-Authenticate: Basic
        end
    end
```

## 2. Login Laravel

**Routes:** `GET/POST /` (middleware `basic.auth`)

```mermaid
sequenceDiagram
    actor User
    participant Auth as AuthController
    participant RL as RateLimiter
    participant DB as SQLite users
    participant Mail as SMTP (opsional)

    User->>Auth: POST / { email, password }
    Auth->>RL: attempt 5x per 60s per IP
    alt rate limited
        Auth-->>User: error "Too many request"
    end
    Auth->>Auth: validate email + password min 6
    Auth->>DB: Auth::attempt()
    alt sukses
        opt MAIL_NOTIFICATION
            Auth->>Mail: New Login Notification
        end
        Auth-->>User: redirect /admin (session cookie)
    else gagal
        Auth-->>User: error invalid credentials
    end
```

## 3. Lockscreen

**Route:** `GET /locked` — logout tapi simpan user id di session

```mermaid
sequenceDiagram
    actor User
    participant Auth as AuthController
    participant Session

    User->>Auth: GET /locked
    alt sudah login
        Auth->>Auth: Auth::logout()
    end
    Auth->>Session: push user id, save referer
    Auth-->>User: lockscreen view (unlock = login ulang)
```

## Protected routes

Semua `/admin/*` memakai middleware `auth` (Laravel session).

## Implikasi GoSite

| Legacy | GoSite |
|--------|--------|
| PHP session | JWT atau secure HTTP-only cookie session |
| Rate limit | middleware per-IP (redis/in-memory) |
| Basic auth | tetap opsional di reverse proxy atau middleware |
| Lockscreen | state frontend + endpoint re-auth tanpa full logout flow |

**API minimum:**

```
POST /api/v1/auth/login
POST /api/v1/auth/logout
GET  /api/v1/auth/me
POST /api/v1/auth/unlock   # lockscreen
```
