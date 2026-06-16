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
# output: docs/wiki-export/ (di-gitignore — artifact hasil generate)
# sidebar: `_Sidebar.md` saja (link EN/ID per baris; GitHub Wiki cuma punya satu sidebar global)
```

Saat push/PR mengubah `docs/`, workflow [`.github/workflows/wiki.yml`](../.github/workflows/wiki.yml) menjalankan `make wiki-export` dan memvalidasi hasilnya. **Publish** ke `GoSite.wiki` hanya saat perubahan masuk **`master`** (atau `main`), atau lewat workflow manual dengan opsi **publish**.

**Setup wiki (sekali):** aktifkan Wiki di Settings, buat halaman manual pertama (mis. `Home`), tambah secret **`WIKI_TOKEN`** jika `GITHUB_TOKEN` ditolak.

## Push ke wiki (manual)

```bash
make wiki-export
bash scripts/push-wiki.sh
```

Atau clone wiki repo:

```bash
git clone https://github.com/jahrulnr/GoSite.wiki.git /tmp/gosite.wiki
cp docs/wiki-export/*.md /tmp/gosite.wiki/
cd /tmp/gosite.wiki && git add -A && git commit -m "docs: sync wiki bilingual" && git push
```

## Struktur wiki yang disarankan

| Halaman wiki | Sumber (EN) | Sumber (ID) |
|--------------|-------------|-------------|
| Home | cuplikan README | docs/README_id.md |
| Architecture | architecture.md | architecture_id.md |
| Plugin installer | sequences/19-plugin-installer.md | sequences/19-plugin-installer_id.md |
| Plugin platform (ADR) | architecture/plugin-platform.md | _(EN; diekspor sebagai Plugin-platform-id)_ |
| Template dev plugin | [plugins/_templates/](../plugins/_templates/) | path repo yang sama |

OpenAPI: [`api/openapi.yaml`](../api/openapi.yaml)
