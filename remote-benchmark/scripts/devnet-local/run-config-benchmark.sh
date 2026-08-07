#!/bin/sh
# Run the remote-benchmark suite against an already-running network described
# by a YAML or JSON config, then generate a self-contained HTML report.
set -eu

usage() {
  cat >&2 <<EOF
usage: $(basename "$0") --config PATH [options]

Options:
  --start-account N       first logical account (default: 1)
  --end-account N         last logical account (default: config num_accounts)
  --nonce N               initial sender nonce (default: query sender accounts)
  --probe-batches N       synchronous probe batches passed to bench (default: 1)
  --fund-batch-size N     account funding batch size (default: 200)
  --fund-mode MODE        funding transport: cosmos or eth (default: config mode)
  --validators N          validator count shown in the report (default: endpoint count)
  --testcase NAME         testcase shown in the report (default: config tx_type)
  --output PATH           report path (default: report/<config>-<timestamp>.html)
  --skip-fund             do not fund benchmark accounts
  --skip-check            do not check benchmark account balances
  -h, --help              show this help
EOF
  exit "${1:-1}"
}

require_non_negative_integer() {
  case "$2" in
    ""|*[!0-9]*)
      echo "$1 must be a non-negative integer: $2" >&2
      exit 1
      ;;
  esac
}

CALLER_DIR="$(pwd -P)"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd -P)"
REMOTE_BENCHMARK_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd -P)"
# report/ is generated output, kept alongside the devnet binaries under
# remote-benchmark/local/ rather than moving with this script.
LOCAL_ARTIFACTS_DIR="${REMOTE_BENCHMARK_DIR}/local"

CONFIG_PATH=""
START_ACCOUNT=1
END_ACCOUNT=""
NONCE=""
PROBE_BATCHES=1
FUND_BATCH_SIZE=200
FUND_MODE=""
VALIDATORS=""
TESTCASE=""
REPORT_PATH=""
RUN_FUND=true
RUN_CHECK=true

while [ "$#" -gt 0 ]; do
  case "$1" in
    --config|--start-account|--end-account|--nonce|--probe-batches|--fund-batch-size|--fund-mode|--validators|--testcase|--output)
      if [ "$#" -lt 2 ]; then
        echo "$1 requires a value" >&2
        usage
      fi
      option="$1"
      value="$2"
      shift 2
      case "${option}" in
        --config) CONFIG_PATH="${value}" ;;
        --start-account) START_ACCOUNT="${value}" ;;
        --end-account) END_ACCOUNT="${value}" ;;
        --nonce) NONCE="${value}" ;;
        --probe-batches) PROBE_BATCHES="${value}" ;;
        --fund-batch-size) FUND_BATCH_SIZE="${value}" ;;
        --fund-mode) FUND_MODE="${value}" ;;
        --validators) VALIDATORS="${value}" ;;
        --testcase) TESTCASE="${value}" ;;
        --output) REPORT_PATH="${value}" ;;
      esac
      ;;
    --skip-fund)
      RUN_FUND=false
      shift
      ;;
    --skip-check)
      RUN_CHECK=false
      shift
      ;;
    -h|--help)
      usage 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage
      ;;
  esac
done

if [ -z "${CONFIG_PATH}" ]; then
  echo "--config is required" >&2
  usage
fi
case "${CONFIG_PATH}" in
  /*) ;;
  *) CONFIG_PATH="${CALLER_DIR}/${CONFIG_PATH}" ;;
esac
if [ ! -f "${CONFIG_PATH}" ]; then
  echo "config file does not exist: ${CONFIG_PATH}" >&2
  exit 1
fi
CONFIG_PATH="$(cd "$(dirname "${CONFIG_PATH}")" && pwd -P)/$(basename "${CONFIG_PATH}")"

require_non_negative_integer "--start-account" "${START_ACCOUNT}"
if [ -n "${NONCE}" ]; then
  require_non_negative_integer "--nonce" "${NONCE}"
fi
require_non_negative_integer "--probe-batches" "${PROBE_BATCHES}"
require_non_negative_integer "--fund-batch-size" "${FUND_BATCH_SIZE}"
if [ -n "${END_ACCOUNT}" ]; then
  require_non_negative_integer "--end-account" "${END_ACCOUNT}"
fi
if [ -n "${VALIDATORS}" ]; then
  require_non_negative_integer "--validators" "${VALIDATORS}"
  if [ "${VALIDATORS}" -eq 0 ]; then
    echo "--validators must be greater than zero" >&2
    exit 1
  fi
fi
case "${FUND_MODE}" in
  ""|cosmos|eth) ;;
  *)
    echo "--fund-mode must be cosmos or eth: ${FUND_MODE}" >&2
    exit 1
    ;;
esac

WORK_DIR="$(mktemp -d)"
cleanup() {
  rm -rf "${WORK_DIR}"
}
trap cleanup EXIT
CONFIG_METADATA="${WORK_DIR}/config-metadata"
BENCH_STATS="${WORK_DIR}/bench-stats.log"

cd "${REMOTE_BENCHMARK_DIR}"
poetry run python - "${CONFIG_PATH}" >"${CONFIG_METADATA}" <<'PY'
import sys

from remote_benchmark.config import load_config

config = load_config(sys.argv[1])
print(config.num_accounts, config.tx_type, len(config.endpoints), sep="\t")
PY
IFS="$(printf '\t')" read -r \
  CONFIG_NUM_ACCOUNTS CONFIG_TX_TYPE CONFIG_ENDPOINTS <"${CONFIG_METADATA}"

END_ACCOUNT="${END_ACCOUNT:-${CONFIG_NUM_ACCOUNTS}}"
VALIDATORS="${VALIDATORS:-${CONFIG_ENDPOINTS}}"
TESTCASE="${TESTCASE:-${CONFIG_TX_TYPE}}"
if [ "${END_ACCOUNT}" -lt "${START_ACCOUNT}" ]; then
  echo "--end-account must be greater than or equal to --start-account" >&2
  exit 1
fi

REPORT_TIMESTAMP="$(date '+%Y%m%d-%H%M%S')"
REPORT_GENERATED_AT="$(date '+%Y-%m-%dT%H:%M:%S%z')"
if [ -z "${REPORT_PATH}" ]; then
  CONFIG_NAME="$(basename "${CONFIG_PATH}")"
  CONFIG_NAME="${CONFIG_NAME%.*}"
  REPORT_PATH="${LOCAL_ARTIFACTS_DIR}/report/${CONFIG_NAME}-${REPORT_TIMESTAMP}.html"
else
  case "${REPORT_PATH}" in
    /*) ;;
    *) REPORT_PATH="${CALLER_DIR}/${REPORT_PATH}" ;;
  esac
fi

echo "=== config: ${CONFIG_PATH} ==="
echo "=== accounts: ${START_ACCOUNT}..${END_ACCOUNT} ==="
if [ "${RUN_FUND}" = true ]; then
  poetry run python - "${CONFIG_PATH}" "${START_ACCOUNT}" "${END_ACCOUNT}" <<'PY'
import sys

import web3

from remote_benchmark.config import load_config
from remote_benchmark.transaction import physical_account_range
from remote_benchmark.utils import gen_account

config = load_config(sys.argv[1])
if config.mode != "eth":
    raise SystemExit

w3 = web3.Web3(web3.HTTPProvider(config.primary.json_rpc))
if "anvil" not in w3.client_version.lower():
    raise SystemExit

start, end = physical_account_range(
    int(sys.argv[2]), int(sys.argv[3]), config.num_txs, config.sender_strategy
)
account_count = end - start + 1
required_balance = (
    account_count * 50 * 10**18
    + account_count * 21_000 * config.gas_price
    + 10**18
)
funding_account = gen_account(config.global_seq, 0)
current_balance = w3.eth.get_balance(funding_account.address)
if current_balance < required_balance:
    response = w3.provider.make_request(
        "anvil_setBalance", [funding_account.address, hex(required_balance)]
    )
    if response.get("error"):
        raise RuntimeError(response["error"])
    print(
        "seeded Anvil funding account",
        funding_account.address,
        "balance:",
        required_balance,
    )
PY

  echo "=== fund ==="
  if [ -n "${FUND_MODE}" ]; then
    poetry run remote-benchmark fund \
      --config "${CONFIG_PATH}" \
      --batch-size "${FUND_BATCH_SIZE}" \
      --mode "${FUND_MODE}" \
      "${START_ACCOUNT}" "${END_ACCOUNT}"
  else
    poetry run remote-benchmark fund \
      --config "${CONFIG_PATH}" \
      --batch-size "${FUND_BATCH_SIZE}" \
      "${START_ACCOUNT}" "${END_ACCOUNT}"
  fi
fi

if [ "${RUN_CHECK}" = true ]; then
  echo "=== check ==="
  poetry run remote-benchmark check \
    --config "${CONFIG_PATH}" "${START_ACCOUNT}" "${END_ACCOUNT}"
fi

echo "=== bench ==="
if [ -n "${NONCE}" ]; then
  poetry run remote-benchmark bench \
    --config "${CONFIG_PATH}" \
    --nonce "${NONCE}" \
    --probe-batches "${PROBE_BATCHES}" \
    "${START_ACCOUNT}" "${END_ACCOUNT}" >"${BENCH_STATS}"
else
  poetry run remote-benchmark bench \
    --config "${CONFIG_PATH}" \
    --probe-batches "${PROBE_BATCHES}" \
    "${START_ACCOUNT}" "${END_ACCOUNT}" >"${BENCH_STATS}"
fi
cat "${BENCH_STATS}"

echo "=== report ==="
poetry run python -m remote_benchmark.report \
  --config "${CONFIG_PATH}" \
  --stats "${BENCH_STATS}" \
  --output "${REPORT_PATH}" \
  --timestamp "${REPORT_GENERATED_AT}" \
  --validators "${VALIDATORS}" \
  --testcase "${TESTCASE}" \
  --start-account "${START_ACCOUNT}" \
  --end-account "${END_ACCOUNT}"

echo "benchmark report: ${REPORT_PATH}"
echo "${TESTCASE} benchmark passed"
