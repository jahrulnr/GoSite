#!/usr/bin/env bash
# Wipe cloud lab storage and recreate gosite (fresh install, no DEMO_SEED).
# Usage: ./scripts/cloud-lab.local.sh reset
#
set -euo pipefail

: "${CLOUD_LAB_HOST:?set CLOUD_LAB_HOST}"
: "${CLOUD_LAB_APP_DIR:=/apps/gosite}"

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
COMPOSE_SRC="$ROOT/scripts/compose.cloud-lab.yml"

echo "==> ensure VM is up"
"$(dirname "$0")/cloud-lab.local.sh" power-on

echo "==> stop gosite and wipe ${CLOUD_LAB_APP_DIR}/data"
ssh "$CLOUD_LAB_HOST" "cd ${CLOUD_LAB_APP_DIR} && sudo docker-compose down 2>/dev/null || true"
ssh "$CLOUD_LAB_HOST" "sudo rm -rf ${CLOUD_LAB_APP_DIR}/data && mkdir -p ${CLOUD_LAB_APP_DIR}/data && sudo chown -R loq:loq ${CLOUD_LAB_APP_DIR}"

echo "==> ship fresh compose (DEMO_SEED=false)"
scp "$COMPOSE_SRC" "${CLOUD_LAB_HOST}:${CLOUD_LAB_APP_DIR}/docker-compose.yml"

echo "==> start gosite (empty storage → gosite init)"
ssh "$CLOUD_LAB_HOST" "cd ${CLOUD_LAB_APP_DIR} && sudo docker-compose up -d"

echo "==> wait for healthy"
ssh "$CLOUD_LAB_HOST" 'for i in $(seq 1 45); do
  if curl -sk --fail -o /dev/null "https://127.0.0.1:1100/" 2>/dev/null; then exit 0; fi
  sleep 2
done
echo "ERROR: panel not reachable"
sudo docker logs --tail 40 gosite 2>/dev/null || true
exit 1'

echo "==> verify fresh state (no demo websites, bundled mcp only)"
site_count=$(curl -sk -c /tmp/gosite-reset-check -X POST "https://127.0.0.1:1100/api/v1/auth/login" \
  -H 'Content-Type: application/json' -d '{"email":"admin@demo.com","password":"123456"}' >/dev/null \
  && curl -sk -b /tmp/gosite-reset-check 'https://127.0.0.1:1100/api/v1/websites' \
  | python3 -c "import sys,json; print(len(json.load(sys.stdin).get('websites',[])))")
if [[ "${site_count:-x}" != "0" ]]; then
  echo "ERROR: expected 0 websites after reset, got ${site_count}"
  exit 1
fi
if ! ssh "$CLOUD_LAB_HOST" 'sudo docker exec gosite /usr/local/bin/gosite plugin list' | grep -q 'gosite/mcp@.*installed'; then
  echo "ERROR: gosite/mcp not installed after fresh init"
  exit 1
fi

echo ""
echo "==> fresh cloud lab ready"
echo "    panel:    https://127.0.0.1:1100/"
echo "    login:    admin@demo.com / 123456 (default admin, not demo seed)"
echo "    websites: empty — create example.bangunsoft.com via UI"
echo "    data:     wiped; DEMO_SEED=false"
