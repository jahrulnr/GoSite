#!/usr/bin/env bash
# One-shot cloud lab VM bootstrap + first GoSite deploy.
# Run from laptop: ./scripts/cloud-lab.local.sh bootstrap
#
set -euo pipefail

: "${CLOUD_LAB_HOST:?set CLOUD_LAB_HOST}"
: "${CLOUD_LAB_APP_DIR:=/apps/gosite}"
: "${PROD_IMAGE:=gosite:1.0.0-dev}"
: "${PROD_TAR:=gosite-cloud-lab.tar.gz}"
: "${PROD_VERSION:=${PROD_IMAGE##*:}}"

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
COMPOSE_SRC="$ROOT/scripts/compose.cloud-lab.yml"

echo "==> ensure VM is up"
"$(dirname "$0")/cloud-lab.local.sh" power-on

echo "==> install Docker + layout on $CLOUD_LAB_HOST"
ssh "$CLOUD_LAB_HOST" "sudo bash -s" <<'REMOTE'
set -euo pipefail
if ! command -v docker >/dev/null 2>&1; then
  export DEBIAN_FRONTEND=noninteractive
  apt-get update
  apt-get install -y docker.io docker-compose curl ca-certificates
  systemctl enable --now docker
fi
usermod -aG docker loq 2>/dev/null || true
docker network inspect services >/dev/null 2>&1 || docker network create services
mkdir -p /apps/gosite/data/www
chown -R loq:loq /apps
REMOTE

echo "==> ship compose to ${CLOUD_LAB_APP_DIR}"
ssh "$CLOUD_LAB_HOST" "mkdir -p ${CLOUD_LAB_APP_DIR}/data"
scp "$COMPOSE_SRC" "${CLOUD_LAB_HOST}:${CLOUD_LAB_APP_DIR}/docker-compose.yml"
ssh "$CLOUD_LAB_HOST" "sed -i 's|image: gosite:.*|image: ${PROD_IMAGE}|' ${CLOUD_LAB_APP_DIR}/docker-compose.yml"

echo "==> build image locally (VERSION=${PROD_VERSION})"
cd "$ROOT"
docker build --network=host --build-arg "VERSION=${PROD_VERSION}" -t "$PROD_IMAGE" .
docker save "$PROD_IMAGE" | gzip > "$PROD_TAR"

echo "==> load image on cloud lab"
scp "$PROD_TAR" "${CLOUD_LAB_HOST}:/tmp/"
ssh "$CLOUD_LAB_HOST" "zcat /tmp/${PROD_TAR} | sudo docker load && rm -f /tmp/${PROD_TAR}"

echo "==> start gosite"
ssh "$CLOUD_LAB_HOST" "cd ${CLOUD_LAB_APP_DIR} && sudo docker-compose up -d --force-recreate"

echo "==> wait for healthy"
ssh "$CLOUD_LAB_HOST" 'for i in $(seq 1 45); do
  status=$(sudo docker inspect -f "{{if .State.Health}}{{.State.Health.Status}}{{else}}starting{{end}}" gosite 2>/dev/null || echo missing)
  if [ "$status" = healthy ]; then exit 0; fi
  if curl -sk --fail -o /dev/null "https://127.0.0.1:1100/" 2>/dev/null; then
    echo "panel reachable (healthcheck may still be starting)"
    exit 0
  fi
  sleep 2
done
echo "ERROR: gosite not healthy"
sudo docker logs --tail 40 gosite 2>/dev/null || true
exit 1'

echo "==> verify bundled plugin seed"
if ! ssh "$CLOUD_LAB_HOST" 'sudo docker exec gosite /usr/local/bin/gosite plugin list' | grep -q 'gosite/mcp@.*installed'; then
  echo "ERROR: gosite/mcp not installed"
  ssh "$CLOUD_LAB_HOST" 'sudo docker exec gosite tail -30 /storage/logs/bootstrap.log 2>/dev/null' || true
  exit 1
fi

rm -f "$PROD_TAR"
echo ""
echo "==> cloud lab ready"
echo "    panel:    https://103.37.124.185:1100/"
echo "    websites: http://103.37.124.185/  https://103.37.124.185/"
echo "    login:  admin@demo.com / 123456"
echo "    ssh:    ./scripts/cloud-lab.local.sh ssh"
echo "    stop:   ./scripts/cloud-lab.local.sh power-off  (when idle)"
