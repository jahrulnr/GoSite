# Wiki GitHub — GoSite

Cara menerbitkan dokumentasi bilingual ke wiki GitHub.

## Bahasa

| Pola | Bahasa | Halaman wiki |
|------|--------|--------------|
| `docs/*.md` | English (utama) | `Architecture.md`, `Home.md`, … |
| `docs/*_id.md` | Indonesia | `Architecture-id.md`, `Home-id.md`, … |

Lihat [LOCALIZATION_id.md](./LOCALIZATION_id.md) — catatan: gunakan [LOCALIZATION.md](./LOCALIZATION.md) versi EN.

## Export

```bash
make wiki-export
# output: docs/wiki-export/
# sidebar: _Sidebar.md (EN), _Sidebar-id.md (ID)
```

## Push ke wiki

```bash
git clone https://github.com/jahrulnr/GoSite.wiki.git /tmp/gosite.wiki
cp docs/wiki-export/*.md /tmp/gosite.wiki/
cd /tmp/gosite.wiki && git add -A && git commit -m "docs: sync wiki bilingual" && git push
```

OpenAPI: [`api/openapi.yaml`](../api/openapi.yaml)
