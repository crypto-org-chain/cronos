#!/usr/bin/env bash
# Spin up a local 1- or 3-validator Cronos devnet on this machine (pystarport,
# via nix-shell) and drive one of the wiki's benchmark test cases against it:
# https://github.com/crypto-org-chain/cronos/wiki/V1.4-Benchmark
#
# Usage: run-benchmark.sh <1|3> <simple-transfer|simple-transfer-unique|erc20-transfer|batch-simple-transfer|batch-simple-transfer-unique|batch-erc20-transfer>
set -euo pipefail

usage() {
  echo "usage: $(basename "$0") <1|3> <simple-transfer|simple-transfer-unique|erc20-transfer|batch-simple-transfer|batch-simple-transfer-unique|batch-erc20-transfer>" >&2
  exit 1
}

VALIDATORS="${1:-}"
TESTCASE="${2:-}"
case "${VALIDATORS}" in
  1|3) ;;
  *) usage ;;
esac
case "${TESTCASE}" in
  simple-transfer|simple-transfer-unique|erc20-transfer|batch-simple-transfer|batch-simple-transfer-unique|batch-erc20-transfer) ;;
  *) usage ;;
esac
if [[ "${VALIDATORS}" != "1" && "${TESTCASE}" == *-unique ]]; then
  echo "${TESTCASE} currently supports only the 1-validator comparison" >&2
  exit 1
fi

LOCAL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REMOTE_BENCHMARK_DIR="$(cd "${LOCAL_DIR}/.." && pwd)"
CRONOS_ROOT="$(cd "${REMOTE_BENCHMARK_DIR}/.." && pwd)"
SHELL_NIX="${CRONOS_ROOT}/integration_tests/shell.nix"
JSONNET_CONFIG="${LOCAL_DIR}/configs/benchmark-${VALIDATORS}val.jsonnet"
BENCH_CONFIG="${LOCAL_DIR}/configs/${VALIDATORS}val-${TESTCASE}.yaml"

# read straight from the config so it always matches num_accounts in
# configs/*.yaml; this is also what patch_erc20_genesis.py funds ERC20
# balance for.
START_ACCOUNT=1
END_ACCOUNT="$(cd "${LOCAL_DIR}/.." && poetry run python -c \
  "import yaml; print(yaml.safe_load(open('${BENCH_CONFIG}'))['num_accounts'])")"
PHYSICAL_END_ACCOUNT="$(cd "${LOCAL_DIR}/.." && poetry run python -c \
  "import yaml; c=yaml.safe_load(open('${BENCH_CONFIG}')); print(c['num_accounts'] * c['num_txs'] if c.get('sender_strategy') == 'unique-per-tx' else c['num_accounts'])")"
if [[ "${PHYSICAL_END_ACCOUNT}" -eq "${END_ACCOUNT}" ]]; then
  FUND_BATCH_SIZE=200
else
  # 2000 native transfers consume 42M gas and stay below the RPC body limit.
  FUND_BATCH_SIZE=2000
fi

BASE_PORT=26650
NODE0_RPC="http://127.0.0.1:$((BASE_PORT + 7))"
NODE0_EVMRPC="http://127.0.0.1:$((BASE_PORT + 1))"

DATA_DIR="$(mktemp -d)"
PYSTARPORT_PID=""

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

echo "=== initializing ${VALIDATORS}-validator devnet in ${DATA_DIR} ==="
nix-shell "${SHELL_NIX}" --run \
  "pystarport init --config '${JSONNET_CONFIG}' --data '${DATA_DIR}' --base_port ${BASE_PORT} --no_remove"

echo "=== injecting ERC20 contract + balances into genesis ==="
cd "${REMOTE_BENCHMARK_DIR}"
poetry run python "${LOCAL_DIR}/patch_erc20_genesis.py" \
  --data-dir "${DATA_DIR}" --num-accounts "${END_ACCOUNT}"

echo "=== starting devnet ==="
nix-shell "${SHELL_NIX}" --run "pystarport start --data '${DATA_DIR}' --quiet" \
  >"${DATA_DIR}/pystarport.log" 2>&1 &
PYSTARPORT_PID=$!

echo "=== waiting for node0 rpc/evmrpc to accept requests ==="
for _ in $(seq 1 120); do
  if curl -s -o /dev/null "${NODE0_RPC}/status" \
    && curl -s -X POST -H 'content-type: application/json' \
      --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
      "${NODE0_EVMRPC}" | grep -q '"result"'; then
    break
  fi
  sleep 1
done

echo "=== funding the remote-benchmark funding account from the devnet's community account ==="
source "${CRONOS_ROOT}/scripts/.env"
# 10x headroom over the physical sender count x 50 CRO each (see the fund
# command's own headroom comment in remote_benchmark/cli.py for why 50 CRO).
FUND_WEI="$(python3 -c "print(${PHYSICAL_END_ACCOUNT} * 500 * 10**18)")"
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
    "value": ${FUND_WEI},
    "nonce": nonce,
    "gas": 21000,
    "gasPrice": 5000000000000,
    "chainId": 777,
}
raw = community.sign_transaction(tx).rawTransaction
w3.eth.send_raw_transaction(raw)
while w3.eth.get_transaction_count(community.address) <= nonce:
    time.sleep(1)
print("fund account", fund_acct.address, "balance:", w3.eth.get_balance(fund_acct.address))
PY

echo "=== fund ==="
# Funding is setup traffic. Use atomic Cosmos batches even when the measured
# benchmark transport is eth: with recheck=false and 20ms blocks, streaming
# sequential raw Ethereum transactions from one funder races CheckTx resets.
poetry run remote-benchmark fund \
  --config "${BENCH_CONFIG}" --mode cosmos --batch-size "${FUND_BATCH_SIZE}" \
  "${START_ACCOUNT}" "${END_ACCOUNT}"

echo "=== check ==="
if [[ "${PHYSICAL_END_ACCOUNT}" -eq "${END_ACCOUNT}" ]]; then
  poetry run remote-benchmark check --config "${BENCH_CONFIG}" "${START_ACCOUNT}" "${END_ACCOUNT}"
else
  # Checking hundreds of thousands of accounts one-by-one would dominate setup.
  poetry run python - <<PY
import web3

from remote_benchmark.utils import gen_account

w3 = web3.Web3(web3.HTTPProvider("${NODE0_EVMRPC}"))
for index in (${START_ACCOUNT}, ${PHYSICAL_END_ACCOUNT}):
    account = gen_account(0, index)
    print(
        index,
        account.address,
        w3.eth.get_transaction_count(account.address),
        w3.eth.get_balance(account.address),
    )
PY
fi

echo "=== bench ==="
# bench generates the load, sends it, and samples from the pre-send block
# through the block where every generated Cosmos envelope has committed. It
# exits nonzero if the full workload does not commit before the timeout,
# unlike gen-txs+send-txs+stats, whose fixed block window can miss the tail.
BENCH_STATS="${DATA_DIR}/bench-stats.log"
poetry run remote-benchmark bench \
  --config "${BENCH_CONFIG}" "${START_ACCOUNT}" "${END_ACCOUNT}" \
  | tee "${BENCH_STATS}"

REPORT_TIMESTAMP="$(date '+%Y%m%d-%H%M%S')"
REPORT_GENERATED_AT="$(date '+%Y-%m-%dT%H:%M:%S%z')"
REPORT_PATH="${LOCAL_DIR}/report/${REPORT_TIMESTAMP}.html"
poetry run python -m remote_benchmark.report \
  --config "${BENCH_CONFIG}" \
  --stats "${BENCH_STATS}" \
  --output "${REPORT_PATH}" \
  --timestamp "${REPORT_GENERATED_AT}" \
  --validators "${VALIDATORS}" \
  --testcase "${TESTCASE}" \
  --start-account "${START_ACCOUNT}" \
  --end-account "${END_ACCOUNT}"

echo "benchmark report: ${REPORT_PATH}"
echo "${VALIDATORS}-validator ${TESTCASE} local benchmark passed"
