> **Bahasa Indonesia:** [Authentication-id](Authentication-id)


Dua lapisan: **HTTP Basic Auth** (edge) dan **session panel** (SQLite).

## GoSite (implementation)

### 1. HTTP Basic Auth (opsional)

**Middleware:** `middleware.BasicAuth` — semua `/api/v1/*`

```mermaid
sequenceDiagram
    actor Browser
    participant NGX as Nginx (optional)
    participant MW as BasicAuth
    participant API as Handler

    Browser->>MW: Request /api/v1/...
    alt AUTH_ENABLE=false
        MW->>API: next()
    else AUTH_ENABLE=true
        alt Authorization: Basic valid
            MW->>API: next()
        else
            MW-->>Browser: 401 WWW-Authenticate
        end
    end
```

| Env | Default |
|-----|---------|
| `AUTH_ENABLE` | `true` |
| `AUTH_USER` / `AUTH_PASS` | `admin` / `admin` |

### 2. Login & session

**Routes:** `/api/v1/auth/*`

```mermaid
sequenceDiagram
    actor User
    participant UI as Preact panel
    participant H as AuthHandler
    participant Svc as auth.Service
    participant DB as SQLite users + sessions

    User->>UI: email + password
    UI->>H: POST /auth/login
    H->>Svc: Login(email, password, remember)
    Svc->>DB: verify bcrypt (users)
    Svc->>DB: INSERT session
    H->>H: Set-Cookie session
    H-->>UI: { token, user }

    loop protected API
        UI->>H: Cookie + Basic auth
        H->>Svc: Me(token)
        H-->>UI: JSON
    end
```

| Method | Path | Auth |
|--------|------|------|
| GET | `/auth/login` | Public — metadata lockscreen, hints |
| POST | `/auth/login` | Public |
| POST | `/auth/logout` | Session |
| GET | `/auth/me` | Session |
| GET | `/auth/lockscreen` | Session |
| POST | `/auth/lock` | Session |
| POST | `/auth/unlock` | Session — re-auth password |

Session stored in SQLite (`sessions` table) with HTTP-only cookie. Lockscreen state in-memory (`auth.Lockscreen`).

Default seed: `admin@demo.com` / `123456`.

### Protected routes

Semua `/api/v1/*` kecuali `GET/POST /auth/login` dan `GET /auth/login` metadata membutuhkan:

1. Basic auth (when enabled)
2. Session cookie valid (`middleware.RequireSession`)

---


## Code

| Paket | Role |
|-------|-------|
| `internal/service/auth` | Login, logout, me, lock/unlock |
| `internal/delivery/http/middleware` | BasicAuth, RequireSession |
| `internal/repository/sqlite` | users, sessions |
