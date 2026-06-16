#!/usr/bin/env bash
# Install a built plugin zip to a local GoSite instance.
#
# Usage:
#   GOSITE_URL=http://127.0.0.1:8080 AUTH_USER=admin AUTH_PASS=secret \
#     ./_shared/scripts/dev-install.sh dist/my-plugin.zip
set -euo pipefail

ZIP="${1:-}"
if [[ -z "$ZIP" || ! -f "$ZIP" ]]; then
  echo "usage: dev-install.sh <artifact.zip>" >&2
  exit 1
fi

GOSITE_URL="${GOSITE_URL:-http://127.0.0.1:8080}"
AUTH_USER="${AUTH_USER:-admin}"
AUTH_PASS="${AUTH_PASS:-123456}"

SHA=$(sha256sum "$ZIP" | awk '{print $1}')
curl -sf -X POST "${GOSITE_URL}/api/v1/plugins/install" \
  -u "${AUTH_USER}:${AUTH_PASS}" \
  -F "artifact=@${ZIP}" \
  -F "sha256=${SHA}"
echo
