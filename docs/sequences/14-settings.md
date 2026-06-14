# Sequence: Settings

Empat area: **profil user**, **php.ini**, **php-fpm.conf**, **www pool**.

**Route:** `GET /admin/setting`

## Load settings page

```mermaid
sequenceDiagram
    actor User
    participant SC as SettingController
    participant FPM as FPM library

    User->>SC: GET /admin/setting
    SC->>FPM: phpConf(), fpmConf(), poolConf()
    SC-->>User: Blade dengan 4 editor
```

## Update profile

**Route:** `POST /admin/setting/update/profile`

```mermaid
sequenceDiagram
    actor User
    participant SC as SettingController
    participant DB as users
    participant Mail

    User->>SC: POST { id, name, email, password? }
    alt password < 6 chars (jika diisi)
        SC-->>User: warning
    end
    SC->>DB: update (bcrypt password jika ada)
    opt MAIL_NOTIFICATION
        SC->>Mail: Profile Updated
    end
    SC-->>User: success
```

## Update PHP config

**Route:** `POST /admin/setting/update/php`

```mermaid
sequenceDiagram
    actor User
    participant SC as SettingController
    participant Cmd as Commander
    participant FPM

    User->>SC: POST php-config body
    SC->>Cmd: php -nc /tmp/test.ini -v
    alt contains "error"
        SC-->>User: error line 1
    end
    SC->>FPM: setPhpConf() → /storage/php/php.ini
    SC-->>User: success
```

## Update FPM & pool

Sama pola:
- `php-fpm -ny /tmp.conf -t` validasi
- Tulis ke `php-fpm.conf` atau `php-fpm.d/www.conf`

**Catatan:** Perubahan FPM/pool tidak auto-restart php-fpm di legacy — pertimbangkan reload di GoSite.

## Implikasi GoSite

```
GET /api/v1/settings
PUT /api/v1/settings/profile
PUT /api/v1/settings/php
PUT /api/v1/settings/fpm
PUT /api/v1/settings/pool
```

Response `GET`:
```json
{
  "user": { "id": 1, "name": "Admin", "email": "admin@demo.com" },
  "php_ini": "...",
  "fpm_conf": "...",
  "pool_conf": "..."
}
```

Setiap `PUT` harus: validate → write → optional reload service.
