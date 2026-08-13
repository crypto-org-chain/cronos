#!/usr/bin/env bash
# Spin up a local 1- or 3-validator Cronos devnet on this machine (pystarport,
# via nix-shell) and drive one of the wiki's benchmark test cases against it:
# https://github.com/crypto-org-chain/cronos/wiki/V1.4-Benchmark
#
# Usage: run-benchmark.sh <1|3|5> <simple-transfer|simple-transfer-unique|erc20-transfer|batch-simple-transfer|batch-simple-transfer-unique|batch-erc20-transfer>
# Set CRONOS_BIN to an executable path to run against a specific cronosd
# binary (e.g. a downloaded release) instead of the nix-built HEAD binary.
# Set KEEP_DATA=1 to leave the devnet data dir (node logs included) behind for
# post-mortem inspection instead of deleting it on exit.
set -euo pipefail

# rpc.max_open_connections in the jsonnet configs is raised past cometbft's
# default (900, itself sized for an assumed 1024 fd ulimit); without a
# matching bump here, a fresh shell's stock ~1024 limit hits EMFILE once
# load approaches that cap instead of the queuing this was meant to fix.
ulimit -n 65536 2>/dev/null || true
# the raise above silently clamps to the shell's hard limit instead of
# failing, so check the actual result rather than the exit code.
if [[ "$(ulimit -n)" -lt 8192 ]]; then
  echo "warning: nofile ulimit is $(ulimit -n) (wanted 65536) - send_batch_size=8000" \
       "may hit \"too many open files\" instead of the queuing this benchmark expects." \
       "Raise your shell's hard limit and retry." >&2
fi

usage() {
  echo "usage: $(basename "$0") <1|3|5> <simple-transfer|simple-transfer-unique|erc20-transfer|batch-simple-transfer|batch-simple-transfer-unique|batch-erc20-transfer>" >&2
  exit 1
}

VALIDATORS="${1:-}"
TESTCASE="${2:-}"
case "${VALIDATORS}" in
  1|3|5) ;;
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

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REMOTE_BENCHMARK_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
CRONOS_ROOT="$(cd "${REMOTE_BENCHMARK_DIR}/.." && pwd)"
SHELL_NIX="${CRONOS_ROOT}/integration_tests/shell.nix"
# report/ and .cache/ are generated output, kept alongside the devnet
# binaries under remote-benchmark/local/ rather than moving with this script.
LOCAL_ARTIFACTS_DIR="${REMOTE_BENCHMARK_DIR}/local"
BENCH_CONFIG="${SCRIPT_DIR}/configs/${VALIDATORS}val-${TESTCASE}.yaml"

# CRONOS_BIN lets this script run against a specific tagged release binary
# (e.g. a downloaded/extracted cronosd) instead of the nix-built HEAD binary.
# mempool.type='app' is app-mempool (v1.8.0-alpha+); older binaries panic on
# it ("unknown mempool type: app", cometbft/node/setup.go), so fall back to a
# config without it below v1.8.0. --async-check-tx is NOT a reliable signal
# for this - it's a generic cometbft/ethermint server flag that already
# exists in v1.7.8's dependency pins, well before cronos wired up app-mempool.
CRONOS_BIN="${CRONOS_BIN:-}"
JSONNET_CONFIG="${SCRIPT_DIR}/configs/benchmark-${VALIDATORS}val.jsonnet"
if [[ -n "${CRONOS_BIN}" ]]; then
  [[ -x "${CRONOS_BIN}" ]] || { echo "CRONOS_BIN=${CRONOS_BIN} is not executable" >&2; exit 1; }
  echo "=== using external cronosd: ${CRONOS_BIN} ==="
  "${CRONOS_BIN}" version --long || true
  CRONOS_BIN_VERSION="$("${CRONOS_BIN}" version 2>/dev/null | tr -d 'v[:space:]')"
  # git describe dev builds off a "v1.8.0-alpha" tag report as
  # "1.8.0-alpha-<N>-g<hash>" - strip that suffix before comparing, or every
  # local build off this tag is wrongly sorted below the release and falls
  # back to the legacy-mempool config (no app-mempool -> ~4x slower CheckTx).
  CRONOS_BIN_VERSION_BASE="$(echo "${CRONOS_BIN_VERSION}" | sed -E 's/-[0-9]+-g[0-9a-f]+$//')"
  if [[ -n "${CRONOS_BIN_VERSION_BASE}" ]] \
    && [[ "${CRONOS_BIN_VERSION_BASE}" != "1.8.0" ]] \
    && [[ "$(printf '%s\n1.8.0\n' "${CRONOS_BIN_VERSION_BASE}" | sort -V | head -1)" == "${CRONOS_BIN_VERSION_BASE}" ]]; then
    JSONNET_CONFIG="${SCRIPT_DIR}/configs/benchmark-${VALIDATORS}val-legacy-mempool.jsonnet"
    echo "=== ${CRONOS_BIN} (v${CRONOS_BIN_VERSION}) predates app-mempool support, using legacy-mempool config ==="
  fi
fi

# Cosmos chain-id is "<name>_<eip155-id>-<version>" (e.g. "cronos_777-1"); the
# EIP-155 id is what every signed tx's chainId must match, or CheckTx rejects
# it. Derived from the selected jsonnet config rather than hardcoded, so
# editing that config's chain_id doesn't silently desync from the txs
# remote_benchmark signs.
COSMOS_CHAIN_ID="$(grep -o "chain_id: '[^']*'" "${JSONNET_CONFIG}" | head -1 | sed -E "s/.*'([^']*)'/\1/")"
EVM_CHAIN_ID="$(echo "${COSMOS_CHAIN_ID}" | sed -E 's/^.*_([0-9]+)-[0-9]+$/\1/')"
if [[ -z "${EVM_CHAIN_ID}" ]]; then
  echo "could not derive EVM chain-id from ${JSONNET_CONFIG}'s chain_id (${COSMOS_CHAIN_ID})" >&2
  exit 1
fi

# read straight from the config so it always matches num_accounts in
# configs/*.yaml; this is also what patch_erc20_genesis.py funds ERC20
# balance for.
START_ACCOUNT=1
END_ACCOUNT="$(cd "${REMOTE_BENCHMARK_DIR}" && poetry run python -c \
  "import yaml; print(yaml.safe_load(open('${BENCH_CONFIG}'))['num_accounts'])")"
PHYSICAL_END_ACCOUNT="$(cd "${REMOTE_BENCHMARK_DIR}" && poetry run python -c \
  "import yaml; c=yaml.safe_load(open('${BENCH_CONFIG}')); print(c['num_accounts'] * c['num_txs'] if c.get('sender_strategy') == 'unique-per-tx' else c['num_accounts'])")"
BENCH_CONFIG_CHAIN_ID="$(cd "${REMOTE_BENCHMARK_DIR}" && poetry run python -c \
  "import yaml; print(yaml.safe_load(open('${BENCH_CONFIG}'))['chain_id'])")"
if [[ "${BENCH_CONFIG_CHAIN_ID}" != "${EVM_CHAIN_ID}" ]]; then
  echo "${BENCH_CONFIG}'s chain_id (${BENCH_CONFIG_CHAIN_ID}) doesn't match" \
       "${JSONNET_CONFIG}'s chain_id (${COSMOS_CHAIN_ID}); update the yaml config" >&2
  exit 1
fi

BASE_PORT=26650
NODE0_RPC="http://127.0.0.1:$((BASE_PORT + 7))"
NODE0_EVMRPC="http://127.0.0.1:$((BASE_PORT + 1))"

# A leftover cronosd from a killed prior run (its cleanup trap can't reach it -
# pystarport execs it with a relative --home, so no absolute path to pkill -f
# on) would otherwise squat on these fixed ports and answer every check below
# with its own stale, already-loaded chain state instead of a fresh one. Each
# validator i gets its own base_port (BASE_PORT + i*10, pystarport's own
# convention - see pystarport/cluster.py's process_config), so a 3/5-validator
# run must check every validator's rpc port, not just node0's.
for ((i = 0; i < VALIDATORS; i++)); do
  NODE_RPC_PORT=$((BASE_PORT + i * 10 + 7))
  if lsof -nP -iTCP:"${NODE_RPC_PORT}" -sTCP:LISTEN >/dev/null 2>&1; then
    echo "port ${NODE_RPC_PORT} is already in use - a leftover devnet from a" \
         "prior run is still listening; find and kill it before retrying" >&2
    lsof -nP -iTCP:"${NODE_RPC_PORT}" -sTCP:LISTEN >&2
    exit 1
  fi
done

# Genesis init + the ERC20/native-balance patch produce identical output for a
# given config + patch script + mnemonics, so cache and reuse them across runs
# instead of redoing that setup work (including live funding txs) every time.
# Hash every module that shapes the cached genesis or tx batch, not just the
# entry-point script, so an edit anywhere in that chain invalidates the cache.
CACHE_KEY="$(cat \
  "${JSONNET_CONFIG}" "${BENCH_CONFIG}" "${CRONOS_ROOT}/scripts/.env" \
  "${SCRIPT_DIR}/patch_erc20_genesis.py" \
  "${REMOTE_BENCHMARK_DIR}/remote_benchmark/contracts.py" \
  "${REMOTE_BENCHMARK_DIR}/remote_benchmark/erc20.py" \
  "${REMOTE_BENCHMARK_DIR}/remote_benchmark/utils.py" \
  "${REMOTE_BENCHMARK_DIR}/remote_benchmark/libp2p.py" \
  "${REMOTE_BENCHMARK_DIR}/remote_benchmark/transaction.py" \
  "${REMOTE_BENCHMARK_DIR}/remote_benchmark/cli.py" \
  | shasum -a 256 | cut -c1-16)"
# Two different binaries would otherwise hash identically (same jsonnet, same
# everything else) and could wrongly share a cached genesis if their genesis
# validation/output format differs - including two different nix-built HEAD
# commits, so resolve and hash the binary that will actually run either way.
HASHED_CRONOS_BIN="${CRONOS_BIN}"
if [[ -z "${HASHED_CRONOS_BIN}" ]]; then
  HASHED_CRONOS_BIN="$(nix-shell "${SHELL_NIX}" --run 'command -v cronosd')"
fi
CACHE_KEY="$(printf '%s' "${CACHE_KEY}$(shasum -a 256 "${HASHED_CRONOS_BIN}" | cut -d' ' -f1)" \
  | shasum -a 256 | cut -c1-16)"
CACHE_DIR="${LOCAL_ARTIFACTS_DIR}/.cache/genesis/${VALIDATORS}val-${TESTCASE}-${CACHE_KEY}"
CHAIN_ID="${COSMOS_CHAIN_ID}"

DATA_DIR="$(mktemp -d)"
PYSTARPORT_PID=""
CACHE_TMP=""
CACHE_LOCK_DIR="${CACHE_DIR}.lock"
CACHE_LOCK_HELD=""

kill_descendants() {
  local pid="$1"
  local child
  for child in $(pgrep -P "${pid}" 2>/dev/null || true); do
    kill_descendants "${child}"
  done
  kill -9 "${pid}" 2>/dev/null || true
}

cleanup() {
  # pystarport execs cronosd with a relative --home (cwd-based), so its argv
  # never contains DATA_DIR - a path-based pkill can't find it. Walk the
  # process tree by pid instead, which works regardless of how a child sets
  # its own --home/-c flags. Must walk the tree BEFORE killing pystarport
  # itself: killing it first lets its children (supervisord, cronosd) get
  # reparented/orphaned, so pgrep -P no longer finds them under it and they
  # survive to race the rm -rf below.
  if [[ -n "${PYSTARPORT_PID}" ]]; then
    kill_descendants "${PYSTARPORT_PID}"
  fi
  if [[ -n "${KEEP_DATA:-}" ]]; then
    echo "=== KEEP_DATA set, leaving devnet data at ${DATA_DIR} ==="
  else
    rm -rf "${DATA_DIR}"
  fi
  [[ -n "${CACHE_TMP}" ]] && rm -rf "${CACHE_TMP}"
  # Only release a lock this process itself holds - if we crash mid-spin,
  # before ever winning the mkdir, rmdir-ing unconditionally here could tear
  # down another process's live critical section instead of just our own.
  [[ -n "${CACHE_LOCK_HELD}" ]] && rm -rf "${CACHE_LOCK_DIR}" 2>/dev/null || true
}
# EXIT alone isn't enough: bash blocked on a foreground pipeline (the `poetry
# run ... bench | tee` below) can be torn down by an untrapped SIGTERM/SIGINT
# without ever running the EXIT trap, leaving pystarport/cronosd orphaned.
# Trapping the signals directly and exiting from the handler guarantees
# cleanup still runs (once, since it's idempotent) via the EXIT trap it chains
# into.
trap cleanup EXIT
trap 'exit 143' INT TERM

# macOS has no flock(1), so use mkdir as the mutex: it's atomic (fails if the
# dir already exists), which is exactly what a lock needs. Holding it across
# the whole read-or-populate section (not just the final publish step) removes
# the need to guess whether an in-progress CACHE_DIR is a live writer or a
# leftover from a killed one - only one process can ever be in this section
# per cache key. A lock left behind by a SIGKILL'd holder (bypasses the EXIT
# trap) is reclaimed by checking whether its owning pid is still alive, not by
# a fixed timeout - genesis funding for a large num_accounts can legitimately
# run past any fixed threshold, and reclaiming from a live-but-slow holder
# would let two processes populate the same CACHE_DIR at once.
CACHE_LOCK_STALE_S=300
mkdir -p "$(dirname "${CACHE_LOCK_DIR}")"
while ! mkdir "${CACHE_LOCK_DIR}" 2>/dev/null; do
  lock_pid="$(cat "${CACHE_LOCK_DIR}/pid" 2>/dev/null || echo "")"
  if [[ -n "${lock_pid}" ]]; then
    if ! kill -0 "${lock_pid}" 2>/dev/null; then
      echo "=== reclaiming cache lock ${CACHE_LOCK_DIR} held by dead pid ${lock_pid} ===" >&2
      rm -rf "${CACHE_LOCK_DIR}"
      continue
    fi
  else
    # No pid file yet - either the holder is mid-acquire (about to write it)
    # or it died between mkdir and the write. Fall back to a fixed timeout so
    # a died-before-writing holder doesn't wedge every waiter forever.
    lock_mtime="$(stat -f %m "${CACHE_LOCK_DIR}" 2>/dev/null || stat -c %Y "${CACHE_LOCK_DIR}" 2>/dev/null || echo "")"
    if [[ -n "${lock_mtime}" ]] && (( $(date +%s) - lock_mtime > CACHE_LOCK_STALE_S )); then
      echo "=== reclaiming stale cache lock ${CACHE_LOCK_DIR} (>${CACHE_LOCK_STALE_S}s old, no pid) ===" >&2
      rm -rf "${CACHE_LOCK_DIR}"
      continue
    fi
  fi
  sleep 0.2
done
echo "$$" >"${CACHE_LOCK_DIR}/pid"
CACHE_LOCK_HELD=1

if [[ -d "${CACHE_DIR}/${CHAIN_ID}" ]]; then
  echo "=== reusing cached genesis from ${CACHE_DIR} ==="
  cp -R "${CACHE_DIR}/." "${DATA_DIR}/"
else
  echo "=== initializing ${VALIDATORS}-validator devnet in ${DATA_DIR} ==="
  CMD_FLAG=""
  [[ -n "${CRONOS_BIN}" ]] && CMD_FLAG="--cmd '${CRONOS_BIN}'"
  nix-shell "${SHELL_NIX}" --run \
    "pystarport init --config '${JSONNET_CONFIG}' --data '${DATA_DIR}' --base_port ${BASE_PORT} --no_remove ${CMD_FLAG}"

  # pystarport only wires the classic reactor's persistent_peers - it has no
  # idea libp2p exists, so a >1-validator libp2p mesh needs bootstrap_peers
  # derived from each node's node_key.json and patched in by hand.
  echo "=== wiring libp2p bootstrap_peers across ${VALIDATORS} validator(s) ==="
  cd "${REMOTE_BENCHMARK_DIR}"
  poetry run python -m remote_benchmark.libp2p "${DATA_DIR}/${CHAIN_ID}" "${VALIDATORS}" "${BASE_PORT}"

  echo "=== injecting ERC20 contract + native balances into genesis ==="
  cd "${REMOTE_BENCHMARK_DIR}"
  poetry run python "${SCRIPT_DIR}/patch_erc20_genesis.py" \
    --data-dir "${DATA_DIR}" --num-accounts "${END_ACCOUNT}" \
    --fund-accounts "${PHYSICAL_END_ACCOUNT}"

  echo "=== populating genesis cache at ${CACHE_DIR} ==="
  CACHE_TMP="${CACHE_DIR}.tmp.$$"
  mkdir -p "$(dirname "${CACHE_DIR}")"
  rm -rf "${CACHE_TMP}" "${CACHE_DIR}"
  cp -R "${DATA_DIR}" "${CACHE_TMP}"
  mv "${CACHE_TMP}" "${CACHE_DIR}"
  CACHE_TMP=""
fi

rm -rf "${CACHE_LOCK_DIR}"
CACHE_LOCK_HELD=""

echo "=== starting devnet ==="
nix-shell "${SHELL_NIX}" --run "pystarport start --data '${DATA_DIR}' --quiet" \
  >"${DATA_DIR}/pystarport.log" 2>&1 &
PYSTARPORT_PID=$!

echo "=== waiting for node0 rpc/evmrpc to accept requests ==="
for _ in $(seq 1 600); do
  if curl -s -o /dev/null "${NODE0_RPC}/status" \
    && curl -s -X POST -H 'content-type: application/json' \
      --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
      "${NODE0_EVMRPC}" | grep -q '"result"'; then
    break
  fi
  sleep 0.2
done

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
# --txs-cache reuses the same signed batch across runs against this cache
# key's genesis-funded accounts, which always start at nonce 0.
EFFECTIVE_BENCH_CONFIG="${BENCH_CONFIG}"
if [[ "${JSONNET_CONFIG}" == *-legacy-mempool.jsonnet ]]; then
  # The legacy CometBFT mempool validates a tx's sequence at CheckTx against
  # committed on-chain state only - it has no pending-nonce tracking like
  # cronos's v1.8 app-mempool. Every config here already sends one full
  # nonce-round per batch (send_batch_size == num_accounts), but at the
  # default send_interval the next round's CheckTx arrives before the
  # current round commits, so it's rejected - silently, since sends beyond
  # the first probe batch are fire-and-forget broadcast_tx_async. Slow the
  # pacing so each round commits before the next round is sent.
  EFFECTIVE_BENCH_CONFIG="${DATA_DIR}/bench-config-legacy.yaml"
  cd "${REMOTE_BENCHMARK_DIR}"
  poetry run python -c "
import yaml
with open('${BENCH_CONFIG}') as f:
    cfg = yaml.safe_load(f)
cfg['send_interval'] = max(cfg.get('send_interval', 0), 0.05)
with open('${EFFECTIVE_BENCH_CONFIG}', 'w') as f:
    yaml.safe_dump(cfg, f)
"
  echo "=== legacy-mempool pacing: send_interval raised to $(poetry run python -c "import yaml; print(yaml.safe_load(open('${EFFECTIVE_BENCH_CONFIG}'))['send_interval'])") ==="
fi
BENCH_STATS="${DATA_DIR}/bench-stats.log"
poetry run remote-benchmark bench \
  --config "${EFFECTIVE_BENCH_CONFIG}" \
  --txs-cache "${CACHE_DIR}/txs-${START_ACCOUNT}-${END_ACCOUNT}.json" \
  "${START_ACCOUNT}" "${END_ACCOUNT}" \
  | tee "${BENCH_STATS}"

REPORT_TIMESTAMP="$(date '+%Y%m%d-%H%M%S')"
REPORT_GENERATED_AT="$(date '+%Y-%m-%dT%H:%M:%S%z')"
REPORT_PATH="${LOCAL_ARTIFACTS_DIR}/report/${REPORT_TIMESTAMP}.html"
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
