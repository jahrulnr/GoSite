#!/bin/bash
set -euo pipefail

LOG_DIR="/storage/logs"
STARTUP_LOG="$LOG_DIR/bootstrap.log"

echo "--- Set working dir ---"
WDIR=$(pwd)

echo "--- Setup /storage ---"
mkdir -p "$LOG_DIR" /storage/www
touch "$STARTUP_LOG"

# One-time migration from legacy Laravel log path.
LEGACY_LOG_DIR="/storage/laravel/logs"
if [ -d "$LEGACY_LOG_DIR" ] && [ ! -L "$LEGACY_LOG_DIR" ]; then
    echo "--- Migrate legacy logs to $LOG_DIR ---" >> "$STARTUP_LOG"
    shopt -s nullglob
    for f in "$LEGACY_LOG_DIR"/*; do
        base="$(basename "$f")"
        if [ ! -e "$LOG_DIR/$base" ]; then
            mv "$f" "$LOG_DIR/$base" >> "$STARTUP_LOG" 2>&1 || true
        fi
    done
    rmdir "$LEGACY_LOG_DIR" 2>/dev/null || true
    rmdir /storage/laravel 2>/dev/null || true
fi

echo "--- Run gosite init ---"
/usr/local/bin/gosite init >> "$STARTUP_LOG" 2>&1

# Generate self-signed default certificate if nginx needs one and none exists.
DEFAULT_SSL_DIR="/storage/webconfig/ssl/live/default"
if [ ! -f "$DEFAULT_SSL_DIR/cert.pem" ] || [ ! -f "$DEFAULT_SSL_DIR/key.pem" ]; then
    echo "--- Generate self-signed default SSL cert ---"
    mkdir -p "$DEFAULT_SSL_DIR"
    if [ -f "$DEFAULT_SSL_DIR/config.conf" ]; then
        openssl req -x509 -nodes -days 3650 -newkey rsa:2048 \
            -keyout "$DEFAULT_SSL_DIR/key.pem" \
            -out "$DEFAULT_SSL_DIR/cert.pem" \
            -config "$DEFAULT_SSL_DIR/config.conf" \
            >> "$STARTUP_LOG" 2>&1
    else
        openssl req -x509 -nodes -days 3650 -newkey rsa:2048 \
            -keyout "$DEFAULT_SSL_DIR/key.pem" \
            -out "$DEFAULT_SSL_DIR/cert.pem" \
            -subj "/CN=localhost" \
            >> "$STARTUP_LOG" 2>&1
    fi
fi

echo "--- Repair nginx config if needed ---"
/usr/local/bin/gosite nginx-repair >> "$STARTUP_LOG" 2>&1 || echo "WARN: nginx-repair failed, see bootstrap.log" >> "$STARTUP_LOG"

if [ ! -f /www/default/index.html ]; then
    echo "--- Generate default /www ---"
    mkdir -p /www/default/
    if [ -f /storage/webconfig/index.html ]; then
        cp -v /storage/webconfig/index.html /www/default/ >> "$STARTUP_LOG" 2>&1
    fi
fi

if [ -d /var/setup ]; then
    # Move staged configs into their runtime locations, then drop the staging area.
    if [ -d /var/setup/nginx ]; then
        rm -rf /etc/nginx
        mv /var/setup/nginx /etc/nginx
    fi
    if [ -d /var/setup/webconfig ]; then
        cp -a /var/setup/webconfig/. /storage/webconfig/ 2>/dev/null || true
    fi
    rm -vr /var/setup >> "$STARTUP_LOG" 2>&1 || true
fi

# Substitute public HTTPS/QUIC port for Alt-Svc (host-mapped port, default 443).
PUBLIC_HTTPS_PORT="${PUBLIC_HTTPS_PORT:-443}"
if [ -d /etc/nginx ]; then
    find /etc/nginx -type f -name '*.conf' -exec sed -i "s/__PUBLIC_HTTPS_PORT__/${PUBLIC_HTTPS_PORT}/g" {} +
fi

echo "--- Mounting FSTAB ---"
/run/fstab_mounter.sh

echo "--- Start nginx ---"
if ! /usr/sbin/nginx -c /etc/nginx/nginx.conf >> "$STARTUP_LOG" 2>&1; then
    echo "WARN: nginx start failed, see bootstrap.log" >> "$STARTUP_LOG"
fi

echo "--- Start gosite ---"
exec /usr/local/bin/gosite serve
