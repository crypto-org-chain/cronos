#!/usr/bin/env bash
# Deploy/manage a patched pystarport chain-dir across the remote hosts listed
# in hosts.env (or HOSTS_ENV, for a different cluster size/file).
#
# Usage: deploy.sh <tag> <libs|push|start|health|logs|stop|wipe> [args...]
#
#   libs   copy librocksdb/libsnappy from node0 to the other hosts (skip for
#          v1.7.8, which is statically linked and needs neither)
#   push   ship <tag>'s chain-dir (binary + per-node home) to each host
#   start  start cronosd on each host, start flags read from tasks.ini
#   health check /status, /net_info, and genesis sha256 across all hosts
#   logs   tail each host's nodeN.log
#   stop   kill cronosd on each host
#   wipe   remove /home/ubuntu/remote-devnet/<tag> on each host
#
# <tag> is e.g. v178 or v180a - keeps runs from colliding on disk or in a
# systemd-run unit name, so a stale prior run can never poison the next one.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd -P)"
# HOSTS_ENV lets a differently-sized cluster (e.g. hosts-15val.env) reuse this
# script without editing it - everything below is driven by NODE_PUBLIC_IPS'/
# NODE_PRIVATE_IPS' actual length, not a hardcoded node count.
HOSTS_ENV="${HOSTS_ENV:-hosts.env}"
# shellcheck source=hosts.env
source "${SCRIPT_DIR}/${HOSTS_ENV}"
NUM_NODES="${#NODE_PUBLIC_IPS[@]}"

usage() {
  echo "usage: $(basename "$0") <tag> <libs|push|start|health|logs|stop|wipe> [args...]" >&2
  exit 1
}

[[ $# -ge 2 ]] || usage
TAG="$1"
CMD="$2"
shift 2

ssh_to() {
  local ip="$1"; shift
  # shellcheck disable=SC2046
  ssh $(ssh_opts) "${SSH_USER}@${ip}" "$@"
}

scp_to() {
  local ip="$1"; shift
  # shellcheck disable=SC2046
  scp $(ssh_opts) "$@" "${SSH_USER}@${ip}:"
}

cmd_libs() {
  echo "=== staging librocksdb/libsnappy from node1 ==="
  local staging="/tmp/remote-devnet-libs"
  rm -rf "${staging}" && mkdir -p "${staging}"
  ssh_to "${NODE_PUBLIC_IPS[0]}" "cat ${NODE1_ROCKSDB_LIB}" > "${staging}/librocksdb.so.10.10.1"
  ssh_to "${NODE_PUBLIC_IPS[0]}" "cat ${NODE1_SNAPPY_LIB}" > "${staging}/libsnappy.so.1"

  for ((i = 1; i < NUM_NODES; i++)); do
    local ip="${NODE_PUBLIC_IPS[$i]}"
    echo "=== pushing libs to node${i} (${ip}) ==="
    ssh_to "${ip}" "mkdir -p ${REMOTE_ROOT}/lib"
    scp $(ssh_opts) "${staging}"/*.so.* "${SSH_USER}@${ip}:${REMOTE_ROOT}/lib/"
    ssh_to "${ip}" "ln -sf librocksdb.so.10.10.1 ${REMOTE_ROOT}/lib/librocksdb.so.10 && \
      LD_LIBRARY_PATH=${REMOTE_ROOT}/lib ldd ${REMOTE_ROOT}/bin/${TAG}/cronosd | (grep 'not found' && exit 1 || true)"
  done
  rm -rf "${staging}"
}

# CHAIN_DIR: local patched chain dir, e.g. .../local/.cache/genesis/<hash>/cronos_777-1
cmd_push() {
  local chain_dir="$1" bin_path="$2"
  [[ -d "${chain_dir}" ]] || { echo "not a dir: ${chain_dir}" >&2; exit 1; }
  [[ -f "${bin_path}" ]] || { echo "not a file: ${bin_path}" >&2; exit 1; }

  local staging="/tmp/remote-devnet-push-${TAG}"
  rm -rf "${staging}" && mkdir -p "${staging}"
  cp "${bin_path}" "${staging}/cronosd"

  for ((i = 0; i < NUM_NODES; i++)); do
    local node_dir="${chain_dir}/node${i}"
    [[ -d "${node_dir}" ]] || { echo "missing ${node_dir}" >&2; exit 1; }
    tar -C "${chain_dir}" -czf "${staging}/node${i}.tar.gz" "node${i}"
  done

  for ((i = 0; i < NUM_NODES; i++)); do
    local ip="${NODE_PUBLIC_IPS[$i]}"
    echo "=== pushing node${i} + binary to ${ip} ==="
    ssh_to "${ip}" "mkdir -p ${REMOTE_ROOT}/${TAG} ${REMOTE_ROOT}/bin/${TAG}"
    scp $(ssh_opts) "${staging}/node${i}.tar.gz" "${SSH_USER}@${ip}:${REMOTE_ROOT}/${TAG}/"
    scp $(ssh_opts) "${staging}/cronosd" "${SSH_USER}@${ip}:${REMOTE_ROOT}/bin/${TAG}/cronosd"
    ssh_to "${ip}" "chmod +x ${REMOTE_ROOT}/bin/${TAG}/cronosd && \
      tar -C ${REMOTE_ROOT}/${TAG} -xzf ${REMOTE_ROOT}/${TAG}/node${i}.tar.gz && \
      rm ${REMOTE_ROOT}/${TAG}/node${i}.tar.gz"
  done

  echo "=== verifying genesis.json is identical across all ${NUM_NODES} hosts ==="
  local ref=""
  for ((i = 0; i < NUM_NODES; i++)); do
    local ip="${NODE_PUBLIC_IPS[$i]}"
    local sum
    sum="$(ssh_to "${ip}" "sha256sum ${REMOTE_ROOT}/${TAG}/node${i}/config/genesis.json" | cut -d' ' -f1)"
    if [[ -z "${ref}" ]]; then ref="${sum}"; fi
    if [[ "${sum}" != "${ref}" ]]; then
      echo "genesis mismatch: node${i} (${ip}) = ${sum}, expected ${ref}" >&2
      exit 1
    fi
  done
  echo "genesis sha256 ${ref} matches on all ${NUM_NODES} hosts"
  rm -rf "${staging}"
}

# start flags come from the tasks.ini pystarport generated locally, so v1.8's
# --async-check-tx and v1.7.8's absence of it are both honored automatically.
# Started in parallel, not sequentially - a sequential ssh loop across many
# hosts spreads startup over tens of seconds, which widens the lp2p dial-race
# window (reconnect backoff is a fixed 1s->5min ceiling, not configurable).
cmd_start() {
  local start_flags="$1"
  local pids=()
  for ((i = 0; i < NUM_NODES; i++)); do
    local ip="${NODE_PUBLIC_IPS[$i]}"
    echo "=== starting node${i} on ${ip} ==="
    (
      ssh_to "${ip}" "systemd-run --user --unit=cronosd-${TAG} \
        --property=LimitNOFILE=65536 \
        --working-directory=${REMOTE_ROOT}/${TAG}/node${i} \
        --setenv=LD_LIBRARY_PATH=${REMOTE_ROOT}/lib \
        ${REMOTE_ROOT}/bin/${TAG}/cronosd start --home . ${start_flags}" \
        || ssh_to "${ip}" "tmux new-session -d -s cronosd-${TAG} \
          'ulimit -n 65536; cd ${REMOTE_ROOT}/${TAG}/node${i} && \
           LD_LIBRARY_PATH=${REMOTE_ROOT}/lib ${REMOTE_ROOT}/bin/${TAG}/cronosd start --home . ${start_flags}'"
    ) &
    pids+=($!)
  done
  local ok=1
  for pid in "${pids[@]}"; do
    wait "${pid}" || ok=0
  done
  [[ "${ok}" == "1" ]] || { echo "one or more nodes failed to start" >&2; exit 1; }
}

cmd_health() {
  # lp2p's reconnect backoff (fixed 1s->5min ceiling, not configurable) means
  # a node started shortly after its peers can sit under-connected for a
  # while - poll instead of taking a single snapshot, especially at higher
  # NUM_NODES where startup is more spread out.
  local timeout_s="${HEALTH_TIMEOUT_S:-90}" poll_interval_s=5
  local deadline=$((SECONDS + timeout_s))
  while true; do
    local ref_hash="" ok=1
    for ((i = 0; i < NUM_NODES; i++)); do
      local ip="${NODE_PUBLIC_IPS[$i]}"
      local rpc_p; rpc_p="$(rpc_port "${i}")"
      local status
      if ! status="$(curl -sf --max-time 5 "http://${ip}:${rpc_p}/status")"; then
        echo "node${i} (${ip}:${rpc_p}) unreachable" >&2
        ok=0
        continue
      fi
      local catching_up height
      catching_up="$(echo "${status}" | jq -r .result.sync_info.catching_up)"
      height="$(echo "${status}" | jq -r .result.sync_info.latest_block_height)"

      local net_info n_peers
      net_info="$(curl -sf --max-time 5 "http://${ip}:${rpc_p}/net_info")"
      n_peers="$(echo "${net_info}" | jq -r .result.n_peers)"

      local jsonrpc_p; jsonrpc_p="$(jsonrpc_port "${i}")"
      local block_number
      block_number="$(curl -sf --max-time 5 -X POST -H 'content-type: application/json' \
        -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
        "http://${ip}:${jsonrpc_p}" | jq -r .result)"

      echo "node${i} (${ip}): catching_up=${catching_up} height=${height} n_peers=${n_peers} eth_blockNumber=${block_number}"
      if [[ "${catching_up}" != "false" || "${n_peers}" != "$((NUM_NODES - 1))" || -z "${block_number}" || "${block_number}" == "null" ]]; then
        ok=0
      fi
    done
    [[ "${ok}" == "1" ]] && { echo "health check OK"; return 0; }
    [[ "${SECONDS}" -ge "${deadline}" ]] && { echo "health check FAILED (timed out after ${timeout_s}s)" >&2; exit 1; }
    echo "not yet healthy, retrying in ${poll_interval_s}s..." >&2
    sleep "${poll_interval_s}"
  done
}

cmd_logs() {
  for ((i = 0; i < NUM_NODES; i++)); do
    local ip="${NODE_PUBLIC_IPS[$i]}"
    echo "=== node${i} (${ip}) ==="
    ssh_to "${ip}" "journalctl --user -u cronosd-${TAG} --no-pager -n 50 2>/dev/null || \
      tmux capture-pane -pt cronosd-${TAG} 2>/dev/null || echo '(no logs found)'"
  done
}

cmd_stop() {
  for ip in "${NODE_PUBLIC_IPS[@]}"; do
    echo "=== stopping cronosd-${TAG} on ${ip} ==="
    ssh_to "${ip}" "systemctl --user stop cronosd-${TAG} 2>/dev/null; \
      tmux kill-session -t cronosd-${TAG} 2>/dev/null; \
      pkill -f '${REMOTE_ROOT}/bin/${TAG}/cronosd' 2>/dev/null || true"
  done
}

cmd_wipe() {
  for ip in "${NODE_PUBLIC_IPS[@]}"; do
    echo "=== wiping ${REMOTE_ROOT}/${TAG} on ${ip} ==="
    ssh_to "${ip}" "rm -rf ${REMOTE_ROOT}/${TAG}"
  done
}

case "${CMD}" in
  libs) cmd_libs "$@" ;;
  push) cmd_push "$@" ;;
  start) cmd_start "$@" ;;
  health) cmd_health "$@" ;;
  logs) cmd_logs "$@" ;;
  stop) cmd_stop "$@" ;;
  wipe) cmd_wipe "$@" ;;
  *) usage ;;
esac
