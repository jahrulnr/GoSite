# Sequence: Settings

GoSite only implements **user profile update**. Legacy PHP/FPM modules are not ported.

## GoSite (implementation)

```mermaid
sequenceDiagram
    actor User
    participant UI as Settings view
    participant H as SettingsHandler
    participant Svc as settings.Service
    participant Auth as auth.Service
    participant DB as users

    User->>UI: Edit name, email, password
    UI->>H: PUT /settings/profile
    H->>Svc: UpdateProfile
    Svc->>DB: bcrypt password if set
    H-->>UI: { id, name, email }
```

### API

| Method | Path | Status |
|--------|------|--------|
| PUT | `/api/v1/settings/profile` | ✅ Implemented |

Current user profile is read via `GET /auth/me`.

### Validation

- Name & email required
- Password optional; minimum 6 characters when set
- bcrypt hash (compatible Laravel `$2y$` prefix)

---

## Legacy BangunSite (not ported)

<details>
<summary>PHP ini, php-fpm, pool editor</summary>

| Legacy route | GoSite |
|--------------|--------|
| `POST /admin/setting/update/php` | ❌ Dropped — panel without PHP |
| `POST /admin/setting/update/fpm` | ❌ Dropped |
| `POST /admin/setting/update/pool` | ❌ Dropped |

Legacy BangunSite edited `/storage/php/*`. GoSite container does not run PHP-FPM for the panel.

</details>

## Code

| File | Role |
|------|-------|
| `internal/service/settings/service.go` | UpdateProfile |
| `internal/delivery/http/handler/settings.go` | HTTP |

UI hints: `GET /ui/meta` → section settings labels.
