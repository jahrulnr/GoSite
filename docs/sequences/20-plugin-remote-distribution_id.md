# Sequence: Distribusi plugin remote (GitHub / GitLab / URL)

Perluasan dari [19-plugin-installer_id.md](./19-plugin-installer_id.md). **Status:** Implemented (gelombang G — **v1.3.1**)

> Checklist implementasi: [20-plugin-remote-distribution-impl.md](./20-plugin-remote-distribution-impl.md)

Lihat dokumen EN lengkap: [20-plugin-remote-distribution.md](./20-plugin-remote-distribution.md).

## Masalah

Installer hari ini hanya **upload zip** / paste manifest. Belum ada install dari GitHub/GitLab seperti Composer, npm, atau UX mirip `go get`.

## Prinsip

Dual path — bukan prebuilt-only, bukan source-only:

| Path | Siapa | Cara |
|------|-------|------|
| **A prefer-release** | Production | Zip release dari index → fetch → `Install()` |
| **B build** | Komunitas | `git tag` → Docker builder terisolasi → zip → `Install()` |

Default resolver: coba Path A dulu, fallback Path B jika ada blok `distribution.build`.

## Posisi produk: ekosistem vs kurasi

| Model | Friction | Contoh |
|-------|----------|--------|
| Ekosistem terbuka | Rendah | WordPress, npm, `go get` |
| Marketplace kurasi | Tinggi | Terraform providers |

**Risiko terbesar bukan malware — tapi tidak ada plugin.** Signing onboarding berat (“Symbian-like”) bisa bunuh komunitas. Tapi plugin GoSite = control-plane privileged → tetap butuh **permission prompt** dan capability enforcement.

## Threat model (ringkas)

```text
Integrity  ≠  Trust  ≠  Security
```

- **Integrity:** byte download = byte publisher (digest/signature)
- **Trust:** operator percaya publisher
- **Security:** dampak jahat dibatasi (capability, isolation)

Signature tidak menjamin plugin aman. Investasi utama setelah distribusi: **permission model + runtime isolation**.

## UX panel admin (utama)

Distribusi remote **berpusat di panel**, bukan CLI. CLI (G6) opsional, memakai API yang sama.

### Kondisi UI sekarang (`Plugins.tsx`)

| Area | Sekarang | Kurang |
|------|----------|--------|
| Halaman `/plugins` | Registry + stat + aside detail | Tidak ada browse/install hub |
| Modal Install | Tab Artifact · Manifest JSON | Tidak ada URL / GitHub / GitLab / katalog |
| Tabel | enable/disable/switch | Tidak ada provenance (sumber install) |
| Keyring | Hanya API | Tidak ada UI kelola kunci vendor |

Upload zip **tetap** untuk lingkungan air-gapped.

### Arsitektur informasi (target)

```text
/plugins                 → registry (default)
/plugins/install         → wizard install (Upload · URL · GitHub · GitLab · Katalog)
/plugins/catalog         → cari plugin (G5)
/settings/plugins        → token GitHub/GitLab, status remote install
/plugins/keyring         → kelola kunci Ed25519 vendor
```

Alternatif MVP: perluas modal **Install** jadi wizard lebar tanpa route baru (cukup untuk G1–G2).

### Wizard install — per sumber

1. **Upload** — sama seperti sekarang (file + sha256 opsional)
2. **URL** (G1) — HTTPS + sha256 wajib → tombol **Resolve** → kartu preview → **Install**
3. **GitHub** (G2) — `owner/repo`, combobox tag/release, pilih asset zip (filter arch host)
4. **GitLab** (G3) — sama, path project GitLab
5. **Katalog** (G5) — search, kartu plugin, detail + Install
6. **Manifest JSON** — tab Advanced (tier 0 / dukungan)

### Kartu preview (wajib sebelum Install)

Menampilkan: `id`, versi, tier, signed ✓/✗, hooks, **permissions yang diminta**, `minGoSiteVersion`, ukuran file, sha256, sumber, install path (release/build).

- **Permission prompt wajib** — operator harus mengakui capability sebelum Install; Enable diblok jika belum ack
- Blokir install jika host terlalu lama
- Peringatan jika versi lain masih **enabled** (install ≠ switch)

### Registry & detail

- Kolom baru: **Source** (ikon upload/github/gitlab/url)
- Aside: kartu **Distribution** (repo, tag, URL, digest)
- Aksi **Cek update** (G5+) bila release lebih baru tersedia

### Settings → Plugins

- Toggle remote install (read-only dari env)
- Token GitHub/GitLab + “test connection”
- Daftar host allowlist (read-only)

### Keyring (G1b)

Tabel vendor/keyId — untuk mode **strict** / enterprise. Path B komunitas **tidak wajib** keyring untuk publish pertama.

## Fase backend (ringkas)

G1 URL → G2 GitHub (prefer-release) → **G2b** Docker builder (Path B) → G3 GitLab → G4 katalog → G5 tier-0 git-ref → G6 CLI → **G1c** permission prompt.

**Ship pertama disarankan:** G1 + G2 + G1c (install remote + permission UX). G2b untuk publisher tag-only; G1b untuk enterprise.

## Perubahan dari review GPT

Lihat [logs/gpt/20-review.md](../../logs/gpt/20-review.md).

### Putaran 1 (sudah masuk)

- **Digest** (`resolved_digest`, `artifact_digest`) sebagai identitas — bukan URL
- **Resolve ringan** — preview dari index, tanpa download zip penuh
- **gosite.plugin.json** wajib di G2/G3 (bukan heuristik nama file)
- **Redirect** — allowlist dicek setelah hop (CDN GitHub)
- **Trusted vendor** di UI keyring
- **Install / Activate / Update** — policy eksplisit; install tidak auto-switch
- Host build ditunda sebagai experimental

### Putaran 2 (baru)

- **Source provenance** — `sourceCommit`, `buildTime` di `gosite.plugin.json` + kolom registry (L2 trust)
- **Trust levels** L1 artifact → L2 source link → L3 attestation (future)
- **Trust modes** operator: `strict` (prod) / `community` (opt-in) / `dev`
- **Vendor key rotation** — install-time trust; badge jika key sudah revoked
- **Failure class** `release_integrity_failed` (asset diganti setelah publish)
- **Repo hilang** — plugin terpasang tetap jalan; update gagal saja
- **Catalog** wajib enforce plugin id ownership (G4)
- **Resolve** mengembalikan `supportedPlatforms` saat GOOS/ARCH tidak cocok
- **apiVersion** — host boleh dukung beberapa versi manifest/index bersamaan
- **Publisher migration** — open question (repo pindah owner)

### Putaran 3 (diskusi ekosistem)

- **Integrity ≠ Trust ≠ Security** — threat model eksplisit
- **Ekosistem vs kurasi** — dual path, bukan satu model
- **prefer-release** default prod + **build** fallback (Docker builder G2b)
- **Permission install prompt** (G1c) — safety UX utama, bukan PKI
- **Strict** default production; **community** opt-in
- Publisher Path B: `git tag` saja, tanpa keyring wajib
- Build contract sempit: Go + `gosite/builder:go` saja

### Fase UI

| Fase | Panel |
|------|-------|
| G1 | Tab URL + preview + kolom provenance |
| G2 | Tab GitHub + parser paste URL release |
| G1c | Permission prompt + ack sebelum Install/Enable |
| G3 | Tab GitLab + settings token |
| G5 | Halaman katalog + badge update |
| G1b | UI keyring (enterprise) |

### Putaran 4 (review implementor)

GPT koreksi: banyak hal sudah ada di seq 19/20. Lubang yang tersisa:

- **Operation locking** — mutex per `plugin_id` untuk install/enable/switch/uninstall; `409 operation_in_progress`
- **G2b cleanup** — container dihapus; image builder di-cache; cleanup gagal = warning only
- **Build limits** — `PLUGIN_BUILD_TIMEOUT`, `MEMORY_MB`, `CPU_LIMIT`
- **Path B provenance** — `resolved_commit`, `builder_image_digest`
- **TOCTOU resolve→install** — fail install, tidak auto re-resolve; optional `resolveToken` TTL
- **Tag force-push** — commit aktual dicatat, bukan hanya tag
- **`auth_token_expired`** — failure class terpisah dari `fetch_failed`
- **Capability diff** — algoritma re-ack eksplisit + `permissions_acked_caps` snapshot
- **Install log** — timeline step di panel (resolve → fetch/build → validate → done)
- **Risk rank** — #1 Path B, #2 concurrency, #3 runtime capability enforcement (bukan prompt saja)

## Kriteria sukses (UI)

- [ ] Wizard install dengan preview sebelum commit
- [ ] Permission prompt: capability diakui sebelum Install; Enable gated; re-ack saat capability baru
- [ ] Install log step-by-step terlihat di detail aside
- [ ] Token expired menampilkan pesan Settings yang jelas
- [ ] Tab GitHub tanpa edit JSON manual
- [ ] Provenance terlihat di registry; keyring bisa dikelola dari panel (G1b)
- [ ] Sumber remote disembunyikan jika `PLUGIN_REMOTE_INSTALL=false`
