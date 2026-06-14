#!/bin/bash
set -euo pipefail

echo "--- Set working dir ---"
WDIR=$(pwd)

echo "--- Setup /storage ---"
mkdir -p /storage/laravel/logs >> /storage/app.log 2>&1 || true
mkdir -p /storage/www >> /storage/app.log 2>&1 || true
touch /storage/app.log 2>/dev/null || true

echo "--- Run gosite init ---"
/usr/local/bin/gosite init >> /storage/app.log 2>&1

# Generate self-signed default certificate if nginx needs one and none exists.
DEFAULT_SSL_DIR="/storage/webconfig/ssl/live/default"
if [ ! -f "$DEFAULT_SSL_DIR/cert.pem" ] || [ ! -f "$DEFAULT_SSL_DIR/key.pem" ]; then
    echo "--- Generate self-signed default SSL cert ---"
    mkdir -p "$DEFAULT_SSL_DIR"
    openssl req -x509 -nodes -days 3650 -newkey rsa:2048 \
        -keyout "$DEFAULT_SSL_DIR/key.pem" \
        -out "$DEFAULT_SSL_DIR/cert.pem" \
        -subj "/CN=localhost" \
        >> /storage/app.log 2>&1
fi

if [ ! -f /www/default/index.html ]; then
    echo "--- Generate default /www ---"
    mkdir -p /www/default/
    if [ -f /storage/webconfig/index.html ]; then
        cp -v /storage/webconfig/index.html /www/default/ >> /storage/app.log 2>&1
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
    rm -vr /var/setup >> /storage/app.log 2>&1 || true
fi

echo "--- Mounting FSTAB ---"
/run/fstab_mounter.sh

echo "--- Start Server ---"
exec supervisord -n -c /etc/supervisord.conf
