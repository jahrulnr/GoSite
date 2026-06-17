#!/usr/bin/env bash
# Bootstrap local dev storage: dirs, self-signed TLS, migrations, seed.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
STORAGE="${STORAGE_PATH:-/tmp/gosite-qa/storage}"
TLS_DIR="$STORAGE/webconfig/ssl/live/default"

mkdir -p \
  "$STORAGE/nginx" \
  "$STORAGE/logs" \
  "$STORAGE/webconfig/site.d" \
  "$STORAGE/webconfig/active.d" \
  "$TLS_DIR"

# Repair self-referential www symlink from a prior dev init.
if [[ -L "$STORAGE/www" ]]; then
  link_target="$(readlink "$STORAGE/www" || true)"
  if [[ "$link_target" == "$STORAGE/www" || "$link_target" == "$WEB_PATH" ]]; then
    rm -f "$STORAGE/www"
  fi
fi
mkdir -p "${WEB_PATH:-$STORAGE/www}"

if [[ ! -f "$TLS_DIR/cert.pem" || ! -f "$TLS_DIR/key.pem" ]]; then
  echo "dev-api-setup: generating self-signed TLS cert in $TLS_DIR"
  openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
    -keyout "$TLS_DIR/key.pem" -out "$TLS_DIR/cert.pem" \
    -subj "/CN=localhost" \
    2>/dev/null
fi

if [[ ! -f "$STORAGE/webconfig/nginx.conf" && -d "$ROOT/config/webconfig" ]]; then
  echo "dev-api-setup: copying webconfig templates"
  cp -a "$ROOT/config/webconfig/." "$STORAGE/webconfig/"
fi

export APP_ENV="${APP_ENV:-local}"
export DEMO_SEED="${DEMO_SEED:-true}"
export STORAGE_PATH="$STORAGE"
export DB_DATABASE="${DB_DATABASE:-$STORAGE/db.sqlite}"
export TEMPLATES_DIR="${TEMPLATES_DIR:-$ROOT/config}"
export ETC_DIR="${ETC_DIR:-/tmp/gosite-qa/etc}"
export LETSENCRYPT_DIR="${LETSENCRYPT_DIR:-$ETC_DIR/letsencrypt}"
export WEB_PATH="${WEB_PATH:-$STORAGE/www}"
mkdir -p "$ETC_DIR"

cd "$ROOT"
if [[ ! -f "$ROOT/dist/bundled-plugins/gosite-mcp.zip" ]]; then
  echo "dev-api-setup: building bundled plugins"
  make -C "$ROOT" bundled-plugins
fi
export PLUGIN_BUNDLED_PATH="${PLUGIN_BUNDLED_PATH:-$ROOT/dist/bundled-plugins}"

go run ./cmd/gosite migrate
go run ./cmd/gosite init || true

echo "dev-api-setup: ready ($STORAGE)"
