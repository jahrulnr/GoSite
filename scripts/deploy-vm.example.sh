#!/usr/bin/env bash
# Copy to deploy.local.sh and fill in values. That file is gitignored.
#
#   cp scripts/deploy-vm.example.sh scripts/deploy.local.sh
#   chmod +x scripts/deploy.local.sh
#   ./scripts/deploy.local.sh
#
# One-shot: build image (with bundled plugins) → ship → recreate container → verify seed.
# Ships IMAGE ONLY — never overwrites docker-compose.yml on the VM.
#
set -euo pipefail

: "${VM_HOST:?set VM_HOST e.g. user@host}"
: "${VM_APP_DIR:=/apps/gosite}"
: "${PROD_IMAGE:=gosite:1.0.0}"
: "${PROD_TAR:=gosite.tar.gz}"
: "${PROD_VERSION:=${PROD_IMAGE##*:}}"

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

echo "==> build image (VERSION=${PROD_VERSION}, release — no -dev suffix)"
docker build --network=host --build-arg "VERSION=${PROD_VERSION}" -t "$PROD_IMAGE" .
docker save "$PROD_IMAGE" | gzip > "$PROD_TAR"

echo "==> ship image to $VM_HOST (compose on VM is NOT touched)"
scp "$PROD_TAR" "${VM_HOST}:/tmp/"
ssh "$VM_HOST" "test -f ${VM_APP_DIR}/docker-compose.yml" \
  || { echo "ERROR: ${VM_APP_DIR}/docker-compose.yml missing on VM — create it first"; exit 1; }

echo "==> load image + recreate container (existing compose)"
ssh "$VM_HOST" "zcat /tmp/${PROD_TAR} | sudo docker load && cd ${VM_APP_DIR} && sudo docker compose up -d --force-recreate"

echo "==> wait for container healthy"
ssh "$VM_HOST" 'for i in $(seq 1 45); do
  status=$(sudo docker inspect -f "{{if .State.Health}}{{.State.Health.Status}}{{else}}starting{{end}}" gosite 2>/dev/null || echo missing)
  if [ "$status" = healthy ]; then exit 0; fi
  sleep 2
done
echo "ERROR: gosite container did not become healthy in time"
sudo docker logs --tail 40 gosite 2>/dev/null || true
exit 1'

echo "==> verify bundled gosite/mcp seeded"
if ! ssh "$VM_HOST" 'sudo docker exec gosite /usr/local/bin/gosite plugin list' | grep -q 'gosite/mcp@.*installed'; then
  echo "ERROR: gosite/mcp not installed after deploy"
  echo "--- bootstrap.log (tail) ---"
  ssh "$VM_HOST" 'sudo docker exec gosite tail -20 /storage/logs/bootstrap.log 2>/dev/null' || true
  echo "--- container logs (tail) ---"
  ssh "$VM_HOST" 'sudo docker logs --tail 30 gosite 2>/dev/null' || true
  exit 1
fi

echo "==> done — gosite/mcp installed (disabled). Enable from Plugins panel."
