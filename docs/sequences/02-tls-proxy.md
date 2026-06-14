# Sequence: TLS Proxy Panel

Go reverse proxy yang melayani panel admin di port **8080** dengan TLS.

**Sumber:** `proxy/main.go`

```mermaid
sequenceDiagram
    actor Browser
    participant GoProxy as server-proxy :8080
    participant Laravel as Laravel :8000
    participant SSL as /storage/webconfig/ssl/live/default/

    Browser->>GoProxy: HTTPS request (any path)
    GoProxy->>GoProxy: ListenAndServeTLS(cert.pem, key.pem)
    GoProxy->>GoProxy: Set header X-Site: Go
    GoProxy->>GoProxy: Add header x-https: true
    GoProxy->>Laravel: HTTP proxy → localhost:8000
    Laravel-->>GoProxy: Response
    GoProxy-->>Browser: TLS response
```

## Detail teknis

| Item | Nilai |
|------|-------|
| Listen | `:8080` |
| Upstream | `http://localhost:8000` |
| TLS min version | TLS 1.2 |
| Read timeout | 60s |
| Cert path | `/storage/webconfig/ssl/live/default/` (fallback `../config/webconfig/...`) |

## Implikasi GoSite

- Tidak perlu proxy terpisah: backend Go bisa langsung `ListenAndServeTLS` di :8080
- Header `x-https: true` mungkin dipakai middleware legacy — evaluasi apakah masih diperlukan
- Nginx publik (:80/:443) tetap terpisah dari panel admin
