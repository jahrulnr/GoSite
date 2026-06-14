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

if [ ! -f /www/default/index.html ]; then
    echo "--- Generate default /www ---"
    mkdir -p /www/default/
    if [ -f /storage/webconfig/index.html ]; then
        cp -v /storage/webconfig/index.html /www/default/ >> /storage/app.log 2>&1
    fi
fi

if [ -d /var/setup ]; then
    rm -vr /var/setup >> /storage/app.log 2>&1 || true
fi

echo "--- Mounting FSTAB ---"
/run/fstab_mounter.sh

echo "--- Start Server ---"
exec supervisord -n -c /etc/supervisord.conf
