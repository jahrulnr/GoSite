# Plugin lifecycle (developer view)

## States

```text
installing → installed | install_failed
installed  → enabling → enabled | enable_failed
enabled    → disabling → installed
```

## Install

```http
POST /api/v1/plugins/install
Content-Type: multipart/form-data

artifact=@my-plugin.zip
sha256=<optional expected digest>
```

Host steps: verify digest/signature → parse manifest → compatibility check →
extract to `/storage/plugins/<id>/<version>/` → run tier-1 `validate` entrypoint
→ mark `installed`.

Retry: same `(id, version)` allowed only from `install_failed`.

## Enable

```http
POST /api/v1/plugins/{vendor}/{name}/enable
{ "version": "1.0.0" }
```

Host starts go-plugin runtime, refreshes hook dispatcher, marks `enabled`.

## Switch version

```http
POST /api/v1/plugins/{vendor}/{name}/switch
{ "version": "2.0.0" }
```

Host runs `MigrateConfig`, disables current, enables target. On failure,
re-enable the previous version manually.

## Disable / uninstall

```http
POST /api/v1/plugins/{vendor}/{name}/disable
DELETE /api/v1/plugins/{vendor}/{name}/versions/1.0.0
DELETE /api/v1/plugins/{vendor}/{name}/versions/1.0.0?purge=true
```

Disable first before uninstalling an enabled version.

## Local dev loop

```bash
make build          # dist/*.zip
make install        # POST to local gosite
# panel → Plugins → Enable
# trigger hook (e.g. reload nginx) and watch plugin logs
make switch V=1.1.0 # after bumping manifest version + rebuild
```

## Failure classes (operator)

| `failure_class` | Meaning |
|-----------------|---------|
| `validate_timeout` | Install validate subprocess timed out |
| `start_failed` | Runtime failed to start on enable |
| `hook_refresh_failed` | Dispatcher refresh failed |
| `stop_failed` | Disable could not stop runtime |
| `config_migration_failed` | Switch rejected config migration |
| `compensation_failed` | Partial rollback — manual recovery |
