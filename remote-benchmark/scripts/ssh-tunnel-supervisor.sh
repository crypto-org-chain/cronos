#!/usr/bin/env bash
# Supervise N independent `ssh -L` tunnels to one remote node, health-checking
# each and recycling only the one that wedges.
#
# A wedged SSH-multiplexed TCP connection head-of-line-blocks every channel
# riding it - retries on the same connection hit the same wedge. A fresh
# curl from a fresh connection succeeds instantly during a stall, so the fix
# is a new ssh process for that slot, not anything client-side.
#
# Indexed (not associative) arrays throughout: macOS ships bash 3.2, which
# has no `declare -A`.
#
# Usage:
#   ssh-tunnel-supervisor.sh user@host remote_rpc:remote_json_rpc \
#     local_rpc:local_json_rpc [local_rpc:local_json_rpc ...]
#
# Example (3-tunnel pool matching configs-scratch/remote-1val-simple-transfer.yaml):
#   ssh-tunnel-supervisor.sh ubuntu@3.1.250.44 26657:8545 \
#     18657:18545 18658:18546 18659:18547
set -euo pipefail

if [[ $# -lt 3 ]]; then
  echo "usage: $0 user@host remote_rpc:remote_json_rpc local_rpc:local_json_rpc [...]" >&2
  exit 1
fi

HOST="$1"; shift
REMOTE_PORTS="$1"; shift
REMOTE_RPC_PORT="${REMOTE_PORTS%%:*}"
REMOTE_JSONRPC_PORT="${REMOTE_PORTS##*:}"

HEALTH_INTERVAL_S=5
HEALTH_TIMEOUT_S=5
FAIL_THRESHOLD=2

PID=()
LOCAL_RPC=()
LOCAL_JSONRPC=()
FAILS=()

spawn() {
  local slot="$1"
  ssh -N \
    -L "${LOCAL_RPC[$slot]}:127.0.0.1:${REMOTE_RPC_PORT}" \
    -L "${LOCAL_JSONRPC[$slot]}:127.0.0.1:${REMOTE_JSONRPC_PORT}" \
    "${HOST}" &
  PID[$slot]=$!
  FAILS[$slot]=0
  echo "[$(date +%T)] slot ${slot}: spawned ssh pid ${PID[$slot]} (rpc=${LOCAL_RPC[$slot]} json_rpc=${LOCAL_JSONRPC[$slot]})"
}

healthy() {
  curl -s -m "${HEALTH_TIMEOUT_S}" -o /dev/null "http://127.0.0.1:$1/status"
}

cleanup() {
  for slot in "${!PID[@]}"; do
    kill "${PID[$slot]}" 2>/dev/null || true
  done
}
trap cleanup EXIT INT TERM

slot=0
for pair in "$@"; do
  LOCAL_RPC[$slot]="${pair%%:*}"
  LOCAL_JSONRPC[$slot]="${pair##*:}"
  spawn "$slot"
  slot=$((slot + 1))
done

echo "supervising ${slot} tunnel(s), health-check every ${HEALTH_INTERVAL_S}s (recycle after ${FAIL_THRESHOLD} consecutive failures)..."
while true; do
  sleep "${HEALTH_INTERVAL_S}"
  for s in "${!LOCAL_RPC[@]}"; do
    if healthy "${LOCAL_RPC[$s]}"; then
      FAILS[$s]=0
      continue
    fi
    FAILS[$s]=$((FAILS[$s] + 1))
    echo "[$(date +%T)] slot ${s}: health check failed (${FAILS[$s]}/${FAIL_THRESHOLD}) on port ${LOCAL_RPC[$s]}"
    if [[ "${FAILS[$s]}" -ge "${FAIL_THRESHOLD}" ]]; then
      echo "[$(date +%T)] slot ${s}: recycling ssh pid ${PID[$s]}"
      kill "${PID[$s]}" 2>/dev/null || true
      wait "${PID[$s]}" 2>/dev/null || true
      spawn "$s"
    fi
  done
done
