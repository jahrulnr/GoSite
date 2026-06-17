#!/usr/bin/env bash
# Copy to deploy.local.sh and fill in values. That file is gitignored.
#
#   cp scripts/deploy-vm.example.sh scripts/deploy.local.sh
#   chmod +x scripts/deploy.local.sh
#   ./scripts/deploy.local.sh
#
# Ships IMAGE ONLY — never overwrites docker-compose.yml on the VM.
# Manage compose on the server yourself (e.g. /apps/gosite/docker-compose.yml).
#
set -euo pipefail

: "${VM_HOST:?set VM_HOST e.g. user@host}"
: "${VM_APP_DIR:=/apps/gosite}"
: "${PROD_IMAGE:=gosite:1.0.0}"
: "${PROD_TAR:=gosite.tar.gz}"

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

echo "==> build image"
docker build --network=host -t "$PROD_IMAGE" .
docker save "$PROD_IMAGE" | gzip > "$PROD_TAR"

echo "==> ship image to $VM_HOST (compose on VM is NOT touched)"
scp "$PROD_TAR" "${VM_HOST}:/tmp/"
ssh "$VM_HOST" "test -f ${VM_APP_DIR}/docker-compose.yml" \
  || { echo "ERROR: ${VM_APP_DIR}/docker-compose.yml missing on VM — create it first"; exit 1; }

echo "==> load image + recreate container (existing compose)"
ssh "$VM_HOST" "zcat /tmp/${PROD_TAR} | sudo docker load && cd ${VM_APP_DIR} && sudo docker compose up -d --force-recreate"

echo "==> done — check panel URL from your VM compose ports"
