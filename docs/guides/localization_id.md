# Dokumentasi — Bahasa & konvensi

Dokumentasi GoSite **dwibahasa**:

| Pola file | Bahasa | Peran |
|-----------|--------|-------|
| `*.md` | **English** | Primary — link default & halaman wiki tanpa suffix |
| `*_id.md` | **Indonesia** | Secondary — prose Indonesia, struktur sama |

## Aturan

- Link di file English → `*.md`.
- Link di file `*_id.md` → `*_id.md`.
- Kode, path, API, diagram Mermaid **identik** di kedua bahasa.
- Wiki export: English → `Architecture.md`, Indonesia → `Architecture-id.md`.

## Perintah

```bash
make wiki-export
```

## Menambah halaman

1. Tulis `topik.md` (English).
2. Tulis `topik_id.md` (Indonesia).
3. `make wiki-export` lalu push wiki jika perlu.
