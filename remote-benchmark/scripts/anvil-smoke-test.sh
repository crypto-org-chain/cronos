#!/usr/bin/env bash
# Launch a local Anvil node and drive the remote-benchmark framework's
# eth-mode fund -> check -> gen-txs -> send-txs -> stats flow against it.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ANVIL_PORT=8545
ANVIL_RPC="http://127.0.0.1:${ANVIL_PORT}"
ANVIL_PID=""
TMP_DIR="$(mktemp -d)"
CONFIG_PATH="${TMP_DIR}/anvil-config.yaml"
TXS_PATH="${TMP_DIR}/txs.json"

START_ACCOUNT=1
END_ACCOUNT=5

cleanup() {
  if [[ -n "${ANVIL_PID}" ]] && kill -0 "${ANVIL_PID}" 2>/dev/null; then
    kill "${ANVIL_PID}" 2>/dev/null || true
    wait "${ANVIL_PID}" 2>/dev/null || true
  fi
  rm -rf "${TMP_DIR}"
}
trap cleanup EXIT

echo "starting anvil on port ${ANVIL_PORT}..."
anvil --port "${ANVIL_PORT}" >"${TMP_DIR}/anvil.log" 2>&1 &
ANVIL_PID=$!

echo "waiting for anvil to accept json-rpc calls..."
for _ in $(seq 1 30); do
  if curl -s -o /dev/null -w '' \
    -X POST -H 'content-type: application/json' \
    --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
    "${ANVIL_RPC}"; then
    break
  fi
  sleep 1
done

cat >"${CONFIG_PATH}" <<EOF
endpoints:
  - name: anvil
    rpc: ${ANVIL_RPC}
    json_rpc: ${ANVIL_RPC}
mode: eth
chain_id: 31337
global_seq: 0
tx_type: simple-transfer
num_accounts: 10
num_txs: 5
batch_size: 1
send_batch_size: 50
send_interval: 0.1
EOF

echo "deriving funding account address..."
FUND_ADDRESS="$(cd "${ROOT_DIR}" && poetry run python -c "
from remote_benchmark.utils import gen_account
print(gen_account(0, 0).address)
")"
echo "funding account: ${FUND_ADDRESS}"

echo "setting funding account balance via anvil_setBalance..."
curl -s -X POST -H 'content-type: application/json' \
  --data "{\"jsonrpc\":\"2.0\",\"method\":\"anvil_setBalance\",\"params\":[\"${FUND_ADDRESS}\",\"0x56BC75E2D63100000\"],\"id\":1}" \
  "${ANVIL_RPC}" >/dev/null

cd "${ROOT_DIR}"

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

echo "anvil smoke test passed"
