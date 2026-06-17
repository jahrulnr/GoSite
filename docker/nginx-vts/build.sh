#!/bin/bash
# Rebuild /usr/sbin/nginx with nginx-module-vts, preserving the stock configure flags.
set -euo pipefail

NGINX_VERSION="${NGINX_VERSION:-1.30.2}"
VTS_REF="${VTS_MODULE_REF:-master}"
BUILD_DIR="${BUILD_DIR:-/tmp/nginx-vts-build}"

apt-get update
apt-get install -y --no-install-recommends \
	build-essential \
	ca-certificates \
	git \
	libpcre2-dev \
	libssl-dev \
	wget \
	zlib1g-dev

rm -rf "${BUILD_DIR}"
mkdir -p "${BUILD_DIR}"
cd "${BUILD_DIR}"

wget -q "https://nginx.org/download/nginx-${NGINX_VERSION}.tar.gz"
tar xzf "nginx-${NGINX_VERSION}.tar.gz"
git clone --depth 1 --branch "${VTS_REF}" https://github.com/vozlt/nginx-module-vts.git vts-module 2>/dev/null \
	|| git clone --depth 1 https://github.com/vozlt/nginx-module-vts.git vts-module

cd "nginx-${NGINX_VERSION}"

# Capture flags from the packaged nginx binary shipped in the base image.
# Word-splitting breaks --with-cc-opt='-g -O2 ...'; eval preserves quoted compiler flags.
CONFIGURE_ARGS="$(nginx -V 2>&1 | sed -n 's/^.*configure arguments: //p')"
eval "./configure ${CONFIGURE_ARGS} --add-module=../vts-module"
make -j"$(nproc)"
install -m 755 objs/nginx /usr/sbin/nginx

nginx -V 2>&1 | grep -qi vts

apt-get purge -y build-essential git wget
apt-get autoremove -y
rm -rf "${BUILD_DIR}" /var/lib/apt/lists/*
