#!/usr/bin/env bash
# Export docs/ to bilingual GitHub wiki markdown (EN primary, ID *-id pages).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DOCS="$ROOT/docs"
OUT="$DOCS/wiki-export"
REPO="${GOSITE_REPO:-jahrulnr/GoSite}"
BRANCH="${GOSITE_BRANCH:-master}"
GITHUB="https://github.com/${REPO}"
BLOB="${GITHUB}/blob/${BRANCH}"
RAW="https://raw.githubusercontent.com/${REPO}/${BRANCH}"

mkdir -p "$OUT"

# Drop stale pages so removed topics do not linger in wiki-export (see dev-docs export-wiki.mjs).
find "$OUT" -maxdepth 1 -name '*.md' -delete

copy_page() {
  local src="$1"
  local dst="$2"
  if [[ ! -f "$src" ]]; then
    echo "WARN: missing $src" >&2
    return 1
  fi
  cp "$src" "$dst"
}

doc_path() {
  local base="$1"
  local lang="$2"
  if [[ "$lang" == id ]]; then
    echo "${DOCS}/${base%.md}_id.md"
  else
    echo "${DOCS}/${base}"
  fi
}

seq_path() {
  local name="$1"
  local lang="$2"
  if [[ "$lang" == id ]]; then
    echo "${DOCS}/sequences/${name%.md}_id.md"
  else
    echo "${DOCS}/sequences/${name}"
  fi
}

wiki_name() {
  local base="$1"
  local lang="$2"
  if [[ "$lang" == id ]]; then
    echo "${base}-id"
  else
    echo "$base"
  fi
}

# Rewrite repo-relative links → wiki page names (wiki_suffix "" or "-id").
rewrite_links() {
  local file="$1"
  local suf="${2:-}"
  sed -i \
    -e "s|](\\./architecture\\.md)|](Architecture${suf})|g" \
    -e "s|](\\./architecture_id\\.md)|](Architecture-id)|g" \
    -e "s|](\\./domain-model\\.md)|](Domain-model${suf})|g" \
    -e "s|](\\./domain-model_id\\.md)|](Domain-model-id)|g" \
    -e "s|](\\./api-inventory\\.md)|](API-reference${suf})|g" \
    -e "s|](\\./api-inventory_id\\.md)|](API-reference-id)|g" \
    -e "s|](\\./nginx-repair\\.md)|](Nginx-auto-repair${suf})|g" \
    -e "s|](\\./nginx-repair_id\\.md)|](Nginx-auto-repair-id)|g" \
    -e "s|](\\./wiki\\.md)|](Home${suf})|g" \
    -e "s|](\\./wiki_id\\.md)|](Home-id)|g" \
    -e "s|](\\./README\\.md)|](Home${suf})|g" \
    -e "s|](\\./README_id\\.md)|](Home-id)|g" \
    -e "s|](\\./dev-mount-testing\\.md)|](Development${suf})|g" \
    -e "s|](\\./dev-mount-testing_id\\.md)|](Development-id)|g" \
    -e "s|](\\./sequences/01-container-startup\\.md)|](Container-startup${suf})|g" \
    -e "s|](\\./sequences/01-container-startup_id\\.md)|](Container-startup-id)|g" \
    -e "s|](\\./sequences/02-tls-proxy\\.md)|](Panel-routing${suf})|g" \
    -e "s|](\\./sequences/02-tls-proxy_id\\.md)|](Panel-routing-id)|g" \
    -e "s|](\\./sequences/03-authentication\\.md)|](Authentication${suf})|g" \
    -e "s|](\\./sequences/03-authentication_id\\.md)|](Authentication-id)|g" \
    -e "s|](\\./sequences/04-dashboard\\.md)|](Dashboard${suf})|g" \
    -e "s|](\\./sequences/04-dashboard_id\\.md)|](Dashboard-id)|g" \
    -e "s|](\\./sequences/05-website-create\\.md)|](Website-create${suf})|g" \
    -e "s|](\\./sequences/05-website-create_id\\.md)|](Website-create-id)|g" \
    -e "s|](\\./sequences/06-website-enable-disable\\.md)|](Website-enable-disable${suf})|g" \
    -e "s|](\\./sequences/06-website-enable-disable_id\\.md)|](Website-enable-disable-id)|g" \
    -e "s|](\\./sequences/07-website-nginx-config\\.md)|](Website-nginx-config${suf})|g" \
    -e "s|](\\./sequences/07-website-nginx-config_id\\.md)|](Website-nginx-config-id)|g" \
    -e "s|](\\./sequences/08-website-ssl\\.md)|](SSL-and-Certbot${suf})|g" \
    -e "s|](\\./sequences/08-website-ssl_id\\.md)|](SSL-and-Certbot-id)|g" \
    -e "s|](\\./sequences/09-website-delete\\.md)|](Website-delete${suf})|g" \
    -e "s|](\\./sequences/09-website-delete_id\\.md)|](Website-delete-id)|g" \
    -e "s|](\\./sequences/10-docker\\.md)|](Operations${suf})|g" \
    -e "s|](\\./sequences/11-file-manager\\.md)|](Operations${suf})|g" \
    -e "s|](\\./sequences/12-mount-manager\\.md)|](Operations${suf})|g" \
    -e "s|](\\./sequences/13-cron-jobs\\.md)|](Operations${suf})|g" \
    -e "s|](\\./sequences/14-settings\\.md)|](Operations${suf})|g" \
    -e "s|](\\./sequences/15-logs\\.md)|](Operations${suf})|g" \
    -e "s|](\\./sequences/16-database-viewer\\.md)|](Operations${suf})|g" \
    -e "s|](\\./01-container-startup\\.md)|](Container-startup${suf})|g" \
    -e "s|](\\./02-tls-proxy\\.md)|](Panel-routing${suf})|g" \
    -e "s|](\\./03-authentication\\.md)|](Authentication${suf})|g" \
    -e "s|](\\./04-dashboard\\.md)|](Dashboard${suf})|g" \
    -e "s|](\\./05-website-create\\.md)|](Website-create${suf})|g" \
    -e "s|](\\./06-website-enable-disable\\.md)|](Website-enable-disable${suf})|g" \
    -e "s|](\\./07-website-nginx-config\\.md)|](Website-nginx-config${suf})|g" \
    -e "s|](\\./08-website-ssl\\.md)|](SSL-and-Certbot${suf})|g" \
    -e "s|](\\./09-website-delete\\.md)|](Website-delete${suf})|g" \
    -e "s|](\\./10-docker\\.md)|](Operations${suf})|g" \
    -e "s|](\\./11-file-manager\\.md)|](Operations${suf})|g" \
    -e "s|](\\./12-mount-manager\\.md)|](Operations${suf})|g" \
    -e "s|](\\./13-cron-jobs\\.md)|](Operations${suf})|g" \
    -e "s|](\\./14-settings\\.md)|](Operations${suf})|g" \
    -e "s|](\\./15-logs\\.md)|](Operations${suf})|g" \
    -e "s|](\\./16-database-viewer\\.md)|](Operations${suf})|g" \
    -e "s|](\\./sequences/19-plugin-installer\\.md)|](Plugin-installer${suf})|g" \
    -e "s|](\\./sequences/19-plugin-installer_id\\.md)|](Plugin-installer-id)|g" \
    -e "s|](\\./19-plugin-installer\\.md)|](Plugin-installer${suf})|g" \
    -e "s|](\\./19-plugin-installer_id\\.md)|](Plugin-installer-id)|g" \
    -e "s|](\\./architecture/plugin-platform\\.md)|](Plugin-platform${suf})|g" \
    -e "s|](\\../architecture/plugin-platform\\.md)|](Plugin-platform${suf})|g" \
    -e "s|](docs/architecture/plugin-platform\\.md)|](Plugin-platform${suf})|g" \
    -e "s|](\\../../plugins/_templates/[^)]*)|](${BLOB}/plugins/_templates/)|g" \
    -e "s|](\\../architecture/plugin-platform\\.md)|](Plugin-platform${suf})|g" \
    -e "s|](plugins/_templates/)|](${BLOB}/plugins/_templates/)|g" \
    -e "s|](\\./sequences/18-grafana-lite\\.md)|](Observability${suf})|g" \
    -e "s|](\\./17-splunk-lite\\.md)|](Observability${suf})|g" \
    -e "s|](\\./18-grafana-lite\\.md)|](Observability${suf})|g" \
    -e "s|](\\./backend-modules\\.md)|](Migration${suf})|g" \
    -e "s|](\\./backend-modules_id\\.md)|](Migration-id)|g" \
    -e "s|](\\../api/openapi\\.yaml)|](${BLOB}/api/openapi.yaml)|g" \
    -e "s|](api/openapi\\.yaml)|](${BLOB}/api/openapi.yaml)|g" \
    -e "s|](\\../api/examples/)|](${BLOB}/api/examples/)|g" \
    -e "s|](\\../README\\.md\\([^)]*\\))|](Development${suf}\\1)|g" \
    -e "s|](\\../README\\.md)|](Home${suf})|g" \
    -e "s|](\\../README_id\\.md\\([^)]*\\))|](Development-id\\1)|g" \
    -e "s|](docs/README\\.md\\([^)]*\\))|](${BLOB}/docs/README.md\\1)|g" \
    -e "s|](docs/README_id\\.md\\([^)]*\\))|](${BLOB}/docs/README_id.md\\1)|g" \
    -e "s|](docs/architecture\\.md)|](Architecture${suf})|g" \
    -e "s|](docs/nginx-repair\\.md)|](Nginx-auto-repair${suf})|g" \
    -e "s|](docs/domain-model\\.md)|](Domain-model${suf})|g" \
    -e "s|](docs/api-inventory\\.md)|](API-reference${suf})|g" \
    -e "s|](docs/wiki\\.md)|](Home${suf})|g" \
    -e "s|](docs/sequences/)|](Sequences-index${suf})|g" \
    -e "s|](internal/config/config\\.go)|](${BLOB}/internal/config/config.go)|g" \
    -e "s|](LICENSE)|](${BLOB}/LICENSE)|g" \
    -e "s|](\\../nginx-repair\\.md)|](Nginx-auto-repair${suf})|g" \
    -e "s|](\\../nginx-repair_id\\.md)|](Nginx-auto-repair-id)|g" \
    -e "s|](\\../architecture\\.md)|](Architecture${suf})|g" \
    -e "s|](\\../architecture_id\\.md)|](Architecture-id)|g" \
    -e "s|](\\../api-inventory\\.md)|](API-reference${suf})|g" \
    -e "s|](\\../wiki\\.md)|](Home${suf})|g" \
    -e "s|](\\../sequences/README\\.md)|](Sequences-index${suf})|g" \
    -e "s|](\\../sequences/README_id\\.md)|](Sequences-index-id)|g" \
    -e "s|](docs/screenshots/|](${RAW}/docs/screenshots/|g" \
    -e "s|](\\./migration/backend-modules\\.md)|](Migration${suf})|g" \
    -e "s|](\\./migration/README\\.md)|](Migration${suf})|g" \
    -e "s|](\\./migration/backend-modules_id\\.md)|](Migration-id)|g" \
    -e "s|](\\./migration/README_id\\.md)|](Migration-id)|g" \
    "$file"
}

combine() {
  local dst="$1"
  shift
  : > "$dst"
  for src in "$@"; do
    echo "" >> "$dst"
    echo "---" >> "$dst"
    echo "" >> "$dst"
    cat "$src" >> "$dst"
  done
}

strip_leading_h1() { sed -i '1{/^# /d;}' "$1"; }
strip_leading_blank() { sed -i '/./,$!d' "$1"; }
strip_legacy_sections() {
  perl -i -0777 -pe 's/\n## Legacy[^\n]*\n.*?(?=\n## |\z)/\n/gs' "$1"
}
demote_h1_to_h2() { sed -i 's/^# /## /' "$1"; }

wiki_page_key() {
  local base
  base="$(basename "$1" .md)"
  base="${base%-id}"
  echo "$base"
}

wiki_is_id() {
  [[ "$(basename "$1")" == *-id.md ]]
}

add_lang_banner() {
  local file="$1"
  local base
  base="$(basename "$file" .md)"
  if [[ "$base" == _Sidebar* ]]; then
    return
  fi
  if [[ "$base" == *-id ]]; then
    { echo "> **English:** [${base%-id}](${base%-id})"; echo ""; cat "$file"; } > "${file}.tmp" && mv "${file}.tmp" "$file"
  else
    { echo "> **Bahasa Indonesia:** [${base}-id](${base}-id)"; echo ""; cat "$file"; } > "${file}.tmp" && mv "${file}.tmp" "$file"
  fi
}

wiki_sanitize() {
  local file="$1"
  local key lang
  key="$(wiki_page_key "$file")"
  if wiki_is_id "$file"; then lang=id; else lang=en; fi
  [[ "$key" == _Sidebar ]] && return

  strip_legacy_sections "$file"

  case "$key" in
    Home)
      strip_leading_h1 "$file"
      if [[ "$lang" == en ]]; then
        sed -i 's| (Laravel)||g' "$file"
        sed -i 's/replaces a multi-process Laravel stack with a single Go service/is a single Go service/g' "$file"
        sed -i 's/the legacy panel/that layout/g' "$file"
      fi
      ;;
    API-reference)
      strip_leading_h1 "$file"
      sed -i '/^> \*\*Canonical contract:/d' "$file"
      sed -i '/^Mapping route legacy/d' "$file"
      sed -i '/^Mapping legacy routes/d' "$file"
      if [[ "$lang" == en ]]; then
        sed -i 's/^## Konvensi/## Conventions/' "$file"
        sed -i 's/| Legacy |/| Legacy panel |/g' "$file"
        sed -i 's/| Panel lama |/| Legacy panel |/g' "$file"
        sed -i 's/| Usulan GoSite |/| GoSite |/g' "$file"
        {
          printf '%s\n\n' "REST API GoSite v1."
          printf '%s\n\n' "Contract: [api/openapi.yaml](${BLOB}/api/openapi.yaml) · [api/examples/](${BLOB}/api/examples/)"
          cat "$file"
        } > "${file}.tmp" && mv "${file}.tmp" "$file"
      else
        sed -i 's/^## Konvensi usulan/## Konvensi/' "$file"
        {
          printf '%s\n\n' "REST API GoSite v1."
          printf '%s\n\n' "Kontrak: [api/openapi.yaml](${BLOB}/api/openapi.yaml) · [api/examples/](${BLOB}/api/examples/)"
          cat "$file"
        } > "${file}.tmp" && mv "${file}.tmp" "$file"
      fi
      sed -i '/^$/N;/^\n$/d' "$file"
      strip_leading_blank "$file"
      ;;
    Architecture|Domain-model|Nginx-auto-repair|Container-startup|Panel-routing|Authentication|Dashboard|Website-create|Website-enable-disable|Website-nginx-config|Website-delete|SSL-and-Certbot|Sequences-index|Plugin-installer|Plugin-platform)
      strip_leading_h1 "$file"
      ;;
    Migration|Operations|Observability|Development)
      demote_h1_to_h2 "$file"
      strip_leading_blank "$file"
      sed -i '1{/^---$/d;}' "$file"
      strip_leading_blank "$file"
      ;;
    *)
      strip_leading_h1 "$file"
      ;;
  esac

  if [[ "$lang" == en ]]; then
    sed -i 's/bukan Laravel queue/by Go job worker/g' "$file"
    sed -i 's/Laravel queue/job queue/g' "$file"
  fi
}

build_home() {
  local lang="$1"
  local out="$2"
  if [[ "$lang" == en ]]; then
    {
      sed -n '3p' "$ROOT/README.md" | sed 's| (Laravel)||g'
      echo ""
      sed -n '10,37p' "$ROOT/README.md" | sed \
        -e 's/replaces a multi-process Laravel stack with a single Go service/is a single Go service/g' \
        -e 's/the legacy panel/that layout/g'
      echo ""
      echo "## Getting started"
      echo ""
      echo "[Development](Development) — local setup, Docker, and production stack verification."
      echo ""
      echo "## Documentation"
      echo ""
      echo "| Area | Pages |"
      echo "|------|-------|"
      echo "| Architecture | [Architecture](Architecture) · [Container-startup](Container-startup) · [Panel-routing](Panel-routing) |"
      echo "| Website & SSL | [Website-create](Website-create) · [Nginx-auto-repair](Nginx-auto-repair) · [SSL-and-Certbot](SSL-and-Certbot) |"
      echo "| Operations | [Operations](Operations) · [Observability](Observability) · [Dashboard](Dashboard) |"
      echo "| Extensions | [Plugin-installer](Plugin-installer) · [Plugin-platform](Plugin-platform) · [templates](${BLOB}/plugins/_templates/) |"
      echo "| Reference | [API-reference](API-reference) · [Sequences-index](Sequences-index) · [Migration](Migration) |"
    } > "$out"
  else
    {
      sed -n '3,8p' "$DOCS/README_id.md"
      echo ""
      sed -n '28,38p' "$DOCS/README_id.md"
      echo ""
      echo "## Memulai"
      echo ""
      echo "[Development-id](Development-id) — setup lokal, Docker, dan verifikasi stack produksi."
      echo ""
      echo "## Dokumentasi"
      echo ""
      echo "| Area | Halaman |"
      echo "|------|---------|"
      echo "| Arsitektur | [Architecture-id](Architecture-id) · [Container-startup-id](Container-startup-id) · [Panel-routing-id](Panel-routing-id) |"
      echo "| Website & SSL | [Website-create-id](Website-create-id) · [Nginx-auto-repair-id](Nginx-auto-repair-id) · [SSL-and-Certbot-id](SSL-and-Certbot-id) |"
      echo "| Operasi | [Operations-id](Operations-id) · [Observability-id](Observability-id) · [Dashboard-id](Dashboard-id) |"
      echo "| Ekstensi | [Plugin-installer-id](Plugin-installer-id) · [Plugin-platform-id](Plugin-platform-id) · [template](${BLOB}/plugins/_templates/) |"
      echo "| Referensi | [API-reference-id](API-reference-id) · [Sequences-index-id](Sequences-index-id) · [Migration-id](Migration-id) |"
    } > "$out"
  fi
}

export_lang() {
  local lang="$1"
  local suf=""
  [[ "$lang" == id ]] && suf="-id"

  build_home "$lang" "$OUT/Home${suf}.md"

  copy_page "$(doc_path architecture.md "$lang")" "$OUT/Architecture${suf}.md"
  copy_page "$(doc_path domain-model.md "$lang")" "$OUT/Domain-model${suf}.md"
  copy_page "$(doc_path api-inventory.md "$lang")" "$OUT/API-reference${suf}.md"
  copy_page "$(doc_path nginx-repair.md "$lang")" "$OUT/Nginx-auto-repair${suf}.md"
  copy_page "$(seq_path 02-tls-proxy.md "$lang")" "$OUT/Panel-routing${suf}.md"
  copy_page "$(seq_path 03-authentication.md "$lang")" "$OUT/Authentication${suf}.md"
  copy_page "$(seq_path 04-dashboard.md "$lang")" "$OUT/Dashboard${suf}.md"
  copy_page "$(seq_path 05-website-create.md "$lang")" "$OUT/Website-create${suf}.md"
  copy_page "$(seq_path 06-website-enable-disable.md "$lang")" "$OUT/Website-enable-disable${suf}.md"
  copy_page "$(seq_path 07-website-nginx-config.md "$lang")" "$OUT/Website-nginx-config${suf}.md"
  copy_page "$(seq_path 08-website-ssl.md "$lang")" "$OUT/SSL-and-Certbot${suf}.md"
  copy_page "$(seq_path 09-website-delete.md "$lang")" "$OUT/Website-delete${suf}.md"
  copy_page "$(seq_path 01-container-startup.md "$lang")" "$OUT/Container-startup${suf}.md"
  copy_page "$(seq_path 19-plugin-installer.md "$lang")" "$OUT/Plugin-installer${suf}.md"
  copy_page "$DOCS/architecture/plugin-platform.md" "$OUT/Plugin-platform${suf}.md"
  if [[ "$lang" == id ]]; then
    copy_page "$DOCS/sequences/README_id.md" "$OUT/Sequences-index${suf}.md"
  else
    copy_page "$DOCS/sequences/README.md" "$OUT/Sequences-index${suf}.md"
  fi

  combine "$OUT/Operations${suf}.md" \
    "$(seq_path 10-docker.md "$lang")" \
    "$(seq_path 11-file-manager.md "$lang")" \
    "$(seq_path 12-mount-manager.md "$lang")" \
    "$(seq_path 13-cron-jobs.md "$lang")" \
    "$(seq_path 14-settings.md "$lang")" \
    "$(seq_path 15-logs.md "$lang")" \
    "$(seq_path 16-database-viewer.md "$lang")"

  combine "$OUT/Observability${suf}.md" \
    "$(seq_path 17-splunk-lite.md "$lang")" \
    "$(seq_path 18-grafana-lite.md "$lang")"

  if [[ "$lang" == id ]]; then
    combine "$OUT/Migration${suf}.md" "$DOCS/migration/README_id.md" "$DOCS/migration/backend-modules_id.md"
    combine "$OUT/Development${suf}.md" "$DOCS/dev-mount-testing_id.md"
  else
    combine "$OUT/Migration${suf}.md" "$DOCS/migration/README.md" "$DOCS/migration/backend-modules.md"
    combine "$OUT/Development${suf}.md" "$DOCS/dev-mount-testing.md"
  fi

  {
    echo ""
    echo "---"
    echo ""
    if [[ "$lang" == id ]]; then
      sed -n '/^## Quick start/,/^## API$/p' "$ROOT/README.md" | sed '$d' | head -n 5
      echo "(Detail lengkap: lihat README repo dan halaman Development-id.)"
    else
      sed -n '/^## Quick start/,/^## API$/p' "$ROOT/README.md" | sed '$d'
    fi
  } >> "$OUT/Development${suf}.md"
}

write_sidebars() {
  # GitHub Wiki exposes a single global _Sidebar.md (see dev-docs github-wiki).
  # Do not link to _Sidebar-id — underscore pages are special and that switcher breaks.
  cat > "$OUT/_Sidebar.md" <<EOF
**[Home (EN)](Home) · [Beranda (ID)](Home-id)**

### Core
- Architecture · [EN](Architecture) · [ID](Architecture-id)
- Domain model · [EN](Domain-model) · [ID](Domain-model-id)
- API reference · [EN](API-reference) · [ID](API-reference-id)
- Container startup · [EN](Container-startup) · [ID](Container-startup-id)
- Panel routing · [EN](Panel-routing) · [ID](Panel-routing-id)
- Authentication · [EN](Authentication) · [ID](Authentication-id)

### Websites & Nginx
- Website create · [EN](Website-create) · [ID](Website-create-id)
- Website enable/disable · [EN](Website-enable-disable) · [ID](Website-enable-disable-id)
- Website nginx config · [EN](Website-nginx-config) · [ID](Website-nginx-config-id)
- Website delete · [EN](Website-delete) · [ID](Website-delete-id)
- Nginx auto-repair · [EN](Nginx-auto-repair) · [ID](Nginx-auto-repair-id)
- SSL & Certbot · [EN](SSL-and-Certbot) · [ID](SSL-and-Certbot-id)

### Operations
- Dashboard · [EN](Dashboard) · [ID](Dashboard-id)
- Operations · [EN](Operations) · [ID](Operations-id)
- Observability · [EN](Observability) · [ID](Observability-id)

### Extensions
- Plugin installer · [EN](Plugin-installer) · [ID](Plugin-installer-id)
- Plugin platform · [EN](Plugin-platform) · [ID](Plugin-platform-id)
- Plugin templates · [repo](${BLOB}/plugins/_templates/)

### Other
- Migration · [EN](Migration) · [ID](Migration-id)
- Development · [EN](Development) · [ID](Development-id)
- Sequences index · [EN](Sequences-index) · [ID](Sequences-index-id)
EOF
}

echo "Exporting bilingual wiki to $OUT"

export_lang en
export_lang id
write_sidebars

for f in "$OUT"/*.md; do
  base="$(basename "$f" .md)"
  suf=""
  [[ "$base" == *-id ]] && suf="-id"
  [[ "$base" == _Sidebar* ]] && continue
  rewrite_links "$f" "$suf"
  wiki_sanitize "$f"
  add_lang_banner "$f"
done

count="$(find "$OUT" -maxdepth 1 -name '*.md' | wc -l)"
echo "Done: $count markdown files (EN + ID) in docs/wiki-export/"
ls -1 "$OUT"/*.md | head -20
echo "..."
