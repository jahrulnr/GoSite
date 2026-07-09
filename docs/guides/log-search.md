# Log search syntax

Splunk-style search for the GoSite **Logs** view (`GET /api/v1/query`). This replaces the legacy `field:value` mini-parser — saved queries using the old syntax must be rewritten.

## Quick reference

| Feature | Example |
|---------|---------|
| Implicit AND | `404 GET` |
| AND / OR / NOT | `error OR timeout`, `login NOT failed` |
| Quoted phrase | `"connection refused"` |
| Field match | `status=404`, `action=login` (`:` works as `=`) |
| Wildcard | `status=3*`, `action=website.*` |
| Regex | `/timeout/`, `status=/^3\d{2}$/` |
| Comparison | `status>=300 status<400` |
| Grouping | `(status=301 OR status=302)` |
| Pipes (history only) | `\| head 50`, `\| sort -ts` |

Canonical example on **access** logs:

```spl
status>=399 AND (curl OR status=200)
```

Meaning: HTTP status ≥ 399 **and** (line contains `curl` **or** status is 200).

## Sources and fields

Nginx access lines use the KV `log_format main` in [`config/nginx/custom.d/nginx-log.conf`](../../config/nginx/custom.d/nginx-log.conf).

| Source | Storage | Indexed columns | Notes |
|--------|---------|-----------------|-------|
| **access** | `log_events` | `status_code`, `bytes`, `site`, `ts` | Method, URI, UA live in `raw_preview` |
| **error** | `log_events` | `site`, `ts` | Native nginx error text in `raw_preview` |
| **audit** | `audit_logs` | `user`, `action`, `domain`, `status`, `message`, … | Panel actions, not HTTP |
| **job** | `job_runs` | `job_type`, `name`, `status`, `output`, `error` | Background jobs |
| **all_sources** | merged | per-source | Sources without a field are skipped (0 hits), not an error |

### Access logs (`structured`)

- `status`, `status_code` — HTTP status (comparisons and regex work here)
- `site` — vhost domain
- `message`, `preview` — full line (`raw_preview`)
- Free text (`GET`, `curl`, `/api`) searches `site` and `raw_preview`

### Error logs (`text`)

- Primarily free-text: `error`, `timeout`, `/upstream/`, `NOT`
- No structured `status_code` — `status>=502` does not apply unless the digits appear in the line text

### Audit / job (`structured`)

- `status` means operation result (`ok` / `failed`), not HTTP 404/500
- `status>=399` on audit/job alone returns 0 hits — use `status=failed`, `action=…`, or text in `output`/`error`

## Pipes

| Command | Effect |
|---------|--------|
| `\| head N` | Keep first N events after merge/sort |
| `\| sort field` | Ascending (`ts`, `status`, `source`, `action`, `user`) |
| `\| sort -field` | Descending |

Pipes apply to **History** search only. Live Tail uses the filter expression; pipes are ignored.

## Breaking change

- Legacy `field:value` syntax is **not** supported
- Saved queries in `saved_queries` may fail validation or return no rows until updated
- UI help comes from `GET /api/v1/query/meta` (`syntax_hint`, `syntax_topics`, per-source `quick_filters` / `examples`)

## API

```http
GET /api/v1/query/meta
GET /api/v1/query?source=access&site=example.com&q=status%3E%3D300%20status%3C400&limit=50
GET /api/v1/query/tail?source=access&site=example.com&q=status%3D5*
```

See also [17-splunk-lite](../sequences/17-splunk-lite.md) for architecture and retention.

## Future (not in v1)

Ingest extra access KV fields (`http_method`, `uri_path`, …) into dedicated columns so `method=GET` works without regex on the raw line.
