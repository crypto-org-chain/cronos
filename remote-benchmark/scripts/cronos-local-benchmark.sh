#!/usr/bin/env bash
# Launch the local 3-validator Cronos devnet (pystarport, via nix-shell,
# using integration_tests/configs/benchmark-3val.jsonnet) and drive the
# remote-benchmark framework against it.
#
# Usage: cronos-local-benchmark.sh <evm|batch>
#   evm   - plain EVM JSON-RPC sends (mode: eth, eth_sendRawTransaction)
#   batch - cosmos-wrapped MsgEthereumTx, batched multiple-per-tx (mode: cosmos)
set -euo pipefail

usage() {
  echo "usage: $(basename "$0") <evm|batch>" >&2
  exit 1
}

TX_MODE="${1:-}"
case "${TX_MODE}" in
  evm|batch) ;;
  *) usage ;;
esac

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CRONOS_ROOT="$(cd "${ROOT_DIR}/.." && pwd)"
SHELL_NIX="${CRONOS_ROOT}/integration_tests/shell.nix"
JSONNET_CONFIG="${CRONOS_ROOT}/integration_tests/configs/benchmark-3val.jsonnet"

# Cosmos chain-id is "<name>_<eip155-id>-<version>" (e.g. "cronos_777-1"); the
# EIP-155 id is what every signed tx's chainId must match, or CheckTx rejects
# it. Derived from the jsonnet config rather than hardcoded, so editing that
# config's chain_id doesn't silently desync from the txs this script signs.
COSMOS_CHAIN_ID="$(grep -o "chain_id: '[^']*'" "${JSONNET_CONFIG}" | head -1 | sed -E "s/.*'([^']*)'/\1/")"
EVM_CHAIN_ID="$(echo "${COSMOS_CHAIN_ID}" | sed -E 's/^.*_([0-9]+)-[0-9]+$/\1/')"
if [[ -z "${EVM_CHAIN_ID}" ]]; then
  echo "could not derive EVM chain-id from ${JSONNET_CONFIG}'s chain_id (${COSMOS_CHAIN_ID})" >&2
  exit 1
fi

BASE_PORT=26650
NODE0_RPC="http://127.0.0.1:$((BASE_PORT + 7))"
NODE0_EVMRPC="http://127.0.0.1:$((BASE_PORT + 1))"

DATA_DIR="$(mktemp -d)"
CONFIG_PATH="${DATA_DIR}/remote-benchmark-config.yaml"
TXS_PATH="${DATA_DIR}/txs.json"
PYSTARPORT_PID=""

START_ACCOUNT=1
END_ACCOUNT=10

cleanup() {
  if [[ -n "${PYSTARPORT_PID}" ]] && kill -0 "${PYSTARPORT_PID}" 2>/dev/null; then
    kill "${PYSTARPORT_PID}" 2>/dev/null || true
    wait "${PYSTARPORT_PID}" 2>/dev/null || true
  fi
  # supervisord/cronosd children reference the data dir in their args (e.g.
  # --home/-c flags), so this catches anything the parent kill missed.
  pkill -f "${DATA_DIR}" 2>/dev/null || true
  sleep 1
  pkill -9 -f "${DATA_DIR}" 2>/dev/null || true
  rm -rf "${DATA_DIR}"
}
trap cleanup EXIT

echo "initializing 3-validator devnet in ${DATA_DIR}..."
nix-shell "${SHELL_NIX}" --run \
  "pystarport init --config '${JSONNET_CONFIG}' --data '${DATA_DIR}' --base_port ${BASE_PORT} --no_remove"

echo "starting devnet..."
nix-shell "${SHELL_NIX}" --run "pystarport start --data '${DATA_DIR}' --quiet" \
  >"${DATA_DIR}/pystarport.log" 2>&1 &
PYSTARPORT_PID=$!

echo "waiting for node0 rpc/evmrpc to accept requests..."
for _ in $(seq 1 120); do
  if curl -s -o /dev/null "${NODE0_RPC}/status" \
    && curl -s -X POST -H 'content-type: application/json' \
      --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
      "${NODE0_EVMRPC}" | grep -q '"result"'; then
    break
  fi
  sleep 1
done

echo "funding the remote-benchmark funding account from the devnet's community account..."
source "${CRONOS_ROOT}/scripts/.env"
cd "${ROOT_DIR}"
poetry run python - <<PY
import time
from eth_account import Account
import web3

Account.enable_unaudited_hdwallet_features()
community = Account.from_mnemonic("${COMMUNITY_MNEMONIC}")

from remote_benchmark.utils import gen_account

fund_acct = gen_account(0, 0)
w3 = web3.Web3(web3.HTTPProvider("${NODE0_EVMRPC}"))
nonce = w3.eth.get_transaction_count(community.address)
tx = {
    "to": fund_acct.address,
    "value": 10000000000000000000000,  # 10000 basetcro
    "nonce": nonce,
    "gas": 21000,
    "gasPrice": 5000000000000,
    "chainId": ${EVM_CHAIN_ID},
}
raw = community.sign_transaction(tx).rawTransaction
w3.eth.send_raw_transaction(raw)
while w3.eth.get_transaction_count(community.address) <= nonce:
    time.sleep(1)
print("fund account", fund_acct.address, "balance:", w3.eth.get_balance(fund_acct.address))
PY

echo "writing ${TX_MODE} mode config to ${CONFIG_PATH}..."
if [[ "${TX_MODE}" == "evm" ]]; then
  cat >"${CONFIG_PATH}" <<EOF
endpoints:
  - name: node0
    rpc: http://127.0.0.1:$((BASE_PORT + 7))
    json_rpc: http://127.0.0.1:$((BASE_PORT + 1))
  - name: node1
    rpc: http://127.0.0.1:$((BASE_PORT + 17))
    json_rpc: http://127.0.0.1:$((BASE_PORT + 11))
  - name: node2
    rpc: http://127.0.0.1:$((BASE_PORT + 27))
    json_rpc: http://127.0.0.1:$((BASE_PORT + 21))
mode: eth
chain_id: ${EVM_CHAIN_ID}
evm_denom: basetcro
gas_price: 5000000000000
global_seq: 0
tx_type: simple-transfer
num_accounts: 10
num_txs: 10
batch_size: 1
send_batch_size: 200
send_interval: 0.2
EOF
else
  cat >"${CONFIG_PATH}" <<EOF
endpoints:
  - name: node0
    rpc: http://127.0.0.1:$((BASE_PORT + 7))
    json_rpc: http://127.0.0.1:$((BASE_PORT + 1))
  - name: node1
    rpc: http://127.0.0.1:$((BASE_PORT + 17))
    json_rpc: http://127.0.0.1:$((BASE_PORT + 11))
  - name: node2
    rpc: http://127.0.0.1:$((BASE_PORT + 27))
    json_rpc: http://127.0.0.1:$((BASE_PORT + 21))
mode: cosmos
chain_id: ${EVM_CHAIN_ID}
evm_denom: basetcro
gas_price: 5000000000000
global_seq: 0
tx_type: simple-transfer
msg_version: "1.4"
num_accounts: 10
num_txs: 10
batch_size: 5
send_batch_size: 200
send_interval: 0.2
EOF
fi

echo "=== fund ==="
poetry run remote-benchmark fund --config "${CONFIG_PATH}" "${START_ACCOUNT}" "${END_ACCOUNT}"

echo "=== check ==="
poetry run remote-benchmark check --config "${CONFIG_PATH}" "${START_ACCOUNT}" "${END_ACCOUNT}"

echo "=== gen-txs ==="
poetry run remote-benchmark gen-txs --config "${CONFIG_PATH}" -o "${TXS_PATH}" "${START_ACCOUNT}" "${END_ACCOUNT}"

echo "=== send-txs ==="
poetry run remote-benchmark send-txs --config "${CONFIG_PATH}" "${TXS_PATH}"

echo "=== stats ==="
poetry run remote-benchmark stats --config "${CONFIG_PATH}" --count 20

echo "cronos local benchmark (${TX_MODE} mode) passed"
