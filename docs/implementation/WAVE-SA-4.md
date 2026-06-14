# WAVE SA-4 — Website + Nginx + SSL

## Scope

- `internal/service/website/`, `nginx/`, `ssl/`
- `internal/infra/nginx/`
- Handlers + routes for website/nginx/ssl endpoints

## Required behavior tests

| ID | Test |
|----|------|
| B1 | `TestDelete_CleanFalse_KeepsFiles` |
| B2 | `TestDelete_CleanTrue_RemovesFiles` |
| B3 | `TestToggle_ReloadFail_Rollback` |
| B4 | `TestSSLManual_UpdatesConfig` |
| B5 | `TestCreate_ProxyType_UpstreamInConfig` |
| B6 | `TestNginxConfig_BackupCreated` |

## Gate

- Min **25** test functions across website/nginx/ssl packages
- No endpoint stubs
