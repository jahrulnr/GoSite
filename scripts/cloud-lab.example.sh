#!/usr/bin/env bash
# Cloud lab VM — agent experimentation host (IDCloudHost).
#
# Copy to cloud-lab.local.sh (gitignored) and set CLOUD_LAB_HOST + IDC VM ids:
#
#   cp scripts/cloud-lab.example.sh scripts/cloud-lab.local.sh
#   chmod +x scripts/cloud-lab.local.sh
#
# Usage:
#   ./scripts/cloud-lab.local.sh ssh          # interactive SSH
#   ./scripts/cloud-lab.local.sh status       # IDC API + optional ping
#   ./scripts/cloud-lab.local.sh power-on     # start VM, wait until running
#   ./scripts/cloud-lab.local.sh power-off    # stop VM (save cost when idle)
#   ./scripts/cloud-lab.local.sh bootstrap      # first-time Docker + GoSite deploy
#   ./scripts/cloud-lab.local.sh reset          # wipe data, fresh install (DEMO_SEED=false)
#
# Requires IDCLOUDHOST_API_KEY in ~/.openclaw/.env (see idcloudhost-api skill).
#
set -euo pipefail

: "${CLOUD_LAB_HOST:?set CLOUD_LAB_HOST e.g. user@127.0.0.1}"
: "${IDC_VM_UUID:?set IDC_VM_UUID from idc-vm-list-all.sh}"
: "${IDC_LOCATION_SLUG:?set IDC_LOCATION_SLUG e.g. sgp01}"

IDC_SCRIPTS="${IDC_SCRIPTS:-$HOME/.agents/skills/idcloudhost-api/scripts}"
CMD="${1:-help}"

vm_status() {
  "$IDC_SCRIPTS/idc-api.sh" GET "${IDC_LOCATION_SLUG}/user-resource/vm?uuid=${IDC_VM_UUID}" \
    | jq -r '.status // "unknown"'
}

wait_vm_status() {
  local want="$1" i
  for i in $(seq 1 60); do
    [[ "$(vm_status)" == "$want" ]] && return 0
    sleep 3
  done
  echo "ERROR: VM did not reach status=${want} (last: $(vm_status))" >&2
  return 1
}

wait_ssh() {
  local i
  for i in $(seq 1 40); do
    if ssh -o BatchMode=yes -o ConnectTimeout=5 -o StrictHostKeyChecking=accept-new \
      "$CLOUD_LAB_HOST" 'echo ok' >/dev/null 2>&1; then
      return 0
    fi
    sleep 3
  done
  echo "WARN: SSH not reachable yet at $CLOUD_LAB_HOST" >&2
  return 1
}

case "$CMD" in
  ssh)
    exec ssh "$CLOUD_LAB_HOST"
    ;;
  status)
    echo "host:     $CLOUD_LAB_HOST"
    echo "uuid:     $IDC_VM_UUID"
    echo "location: $IDC_LOCATION_SLUG"
    echo -n "idc:      "
    vm_status
    if ssh -o BatchMode=yes -o ConnectTimeout=5 "$CLOUD_LAB_HOST" 'hostname' 2>/dev/null; then
      echo "ssh:      reachable"
    else
      echo "ssh:      unreachable (VM stopped or still booting)"
    fi
    ;;
  power-on|start|up)
    cur="$(vm_status)"
    if [[ "$cur" == "running" ]]; then
      echo "VM already running"
    else
      echo "==> starting VM $IDC_VM_UUID ($IDC_LOCATION_SLUG)"
      "$IDC_SCRIPTS/idc-vm-start.sh" "$IDC_VM_UUID" "$IDC_LOCATION_SLUG"
      wait_vm_status running
      echo "==> VM running"
    fi
    echo "==> waiting for SSH ($CLOUD_LAB_HOST)"
    wait_ssh || true
    ;;
  bootstrap)
    exec "$(dirname "$0")/cloud-lab-bootstrap.sh"
    ;;
  reset)
    exec "$(dirname "$0")/cloud-lab-reset.sh"
    ;;
  power-off|stop|down)
    cur="$(vm_status)"
    if [[ "$cur" == "stopped" ]]; then
      echo "VM already stopped"
      exit 0
    fi
    echo "==> stopping VM $IDC_VM_UUID ($IDC_LOCATION_SLUG)"
    "$IDC_SCRIPTS/idc-vm-stop.sh" "$IDC_VM_UUID" "$IDC_LOCATION_SLUG"
    wait_vm_status stopped
    echo "==> VM stopped"
    ;;
  help|--help|-h)
    sed -n '2,20p' "$0" | sed 's/^# \{0,1\}//'
    ;;
  *)
    echo "unknown command: $CMD (try: ssh | status | power-on | power-off)" >&2
    exit 1
    ;;
esac
