#!/bin/bash
# Builds cronosd, brings up a two-validator devnet via pystarport, generates a
# devnet_tests config pointing at both nodes, and runs the full devnet_tests
# pytest suite (unit + live) against it. Two nodes are required: the cross-node
# checks (rpc-diff equivalence, app-hash agreement) skip themselves on a
# one-node config, which used to make this job green without running them. Used
# by .github/workflows/devnet-smoke.yml and runnable locally the same way.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

BASE_PORT=26650
DATA_DIR="$(mktemp -d)"
DEVNET_CONFIG="$(mktemp)"
# Floor on live probes that must actually pass, well under the 12 collected so a
# few environment-dependent skips stay allowed, well over 0 so a suite that
# skipped itself wholesale can't pass as green.
MIN_LIVE_PASSED=6

if [ -n "${GITHUB_ENV:-}" ]; then
  echo "DEVNET_SMOKE_DATA_DIR=$DATA_DIR" >> "$GITHUB_ENV"
fi

cleanup() {
  if [ -n "${PYSTARPORT_PID:-}" ] && kill -0 "$PYSTARPORT_PID" 2>/dev/null; then
    kill -TERM -"$PYSTARPORT_PID" 2>/dev/null || true
    # `wait` is a shell builtin, so it can't be run under `timeout`; poll instead.
    for _ in $(seq 1 100); do
      kill -0 "$PYSTARPORT_PID" 2>/dev/null || break
      sleep 0.1
    done
  fi
  if [ "${SMOKE_SUCCEEDED:-false}" = true ]; then
    rm -rf "$DATA_DIR" "$DEVNET_CONFIG"
  fi
}
trap cleanup EXIT

echo "== building cronosd =="
make build LEDGER_ENABLED=false
export PATH="$ROOT_DIR/build:$PATH"

echo "== bringing up two-validator devnet in $DATA_DIR =="
pystarport init \
  --config scripts/cronos-single-devnet.yaml \
  --dotenv .env \
  --data "$DATA_DIR" \
  --base_port "$BASE_PORT" \
  --no_remove
setsid pystarport start --data "$DATA_DIR" --quiet &
PYSTARPORT_PID=$!

# pystarport 0.2.5: validator i gets base_port + i*10 (cluster.py), and within a
# node ports.py maps evmrpc_port = base_port + 1, rpc_port = base_port + 7.
NODE1_BASE_PORT=$((BASE_PORT + 10))
EVMRPC_PORT=$((BASE_PORT + 1))
RPC_PORT=$((BASE_PORT + 7))
EVMRPC_PORT_1=$((NODE1_BASE_PORT + 1))
RPC_PORT_1=$((NODE1_BASE_PORT + 7))

wait_for_evmrpc_port() {
  local port=$1
  for _ in $(seq 1 400); do
    if ! kill -0 "$PYSTARPORT_PID" 2>/dev/null; then
      echo "pystarport process died while waiting for the devnet to start" >&2
      exit 1
    fi
    (exec 3<>"/dev/tcp/127.0.0.1/$port") 2>/dev/null && return 0
    sleep 0.1
  done
  return 1
}

for port in "$EVMRPC_PORT" "$EVMRPC_PORT_1"; do
  echo "== waiting for EVM JSON-RPC on :$port =="
  if ! wait_for_evmrpc_port "$port"; then
    echo "devnet did not open its EVM JSON-RPC port $port within 40s" >&2
    exit 1
  fi
done

# The JSON-RPC server binds before consensus starts, so an open port says nothing
# about the chain making progress. Without this, a devnet stalled at height 0
# fails much later as a pile of confusing per-test timeouts.
wait_for_block_production() {
  local port=$1 height
  for _ in $(seq 1 300); do
    if ! kill -0 "$PYSTARPORT_PID" 2>/dev/null; then
      echo "pystarport process died while waiting for block production" >&2
      exit 1
    fi
    height="$(curl -s --max-time 5 -X POST -H 'Content-Type: application/json' \
      --data '{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}' \
      "http://127.0.0.1:$port" |
      sed -n 's/.*"result"[[:space:]]*:[[:space:]]*"\(0x[0-9a-fA-F]*\)".*/\1/p')"
    if [ -n "$height" ] && [ "$((height))" -ge 1 ]; then
      return 0
    fi
    sleep 0.2
  done
  return 1
}

for port in "$EVMRPC_PORT" "$EVMRPC_PORT_1"; do
  echo "== waiting for block production on :$port =="
  if ! wait_for_block_production "$port"; then
    echo "devnet never produced a block on :$port within 60s — consensus never " \
      "started (check validator voting power and peering)" >&2
    exit 1
  fi
done

cat > "$DEVNET_CONFIG" <<EOF
nodes:
  - name: cronos_777-1-node0
    rpc: tcp://127.0.0.1:$RPC_PORT
    json_rpc: http://127.0.0.1:$EVMRPC_PORT
  - name: cronos_777-1-node1
    rpc: tcp://127.0.0.1:$RPC_PORT_1
    json_rpc: http://127.0.0.1:$EVMRPC_PORT_1
chain_id: 777
EOF

# shellcheck disable=SC1091
source scripts/.env
cd devnet_tests

echo "== installing devnet_tests deps =="
poetry install

echo "== deriving funded key from COMMUNITY_MNEMONIC =="
export DEVNET_FUNDED_KEY
# eth_account is a poetry-managed dep of devnet_tests, not a system package.
DEVNET_FUNDED_KEY="$(poetry run python3 -c '
import os
from eth_account import Account
Account.enable_unaudited_hdwallet_features()
print(Account.from_mnemonic(os.environ["COMMUNITY_MNEMONIC"]).key.hex())
')"

# Every live probe depends on the funded_account fixture, which skips itself when
# the key is empty. A silent derivation failure would skip the whole live suite
# and still exit 0 — the exact always-green outcome this script exists to catch.
if ! [[ "$DEVNET_FUNDED_KEY" =~ ^(0x)?[0-9a-fA-F]{64}$ ]]; then
  echo "DEVNET_FUNDED_KEY is not a 32-byte hex private key (got ${#DEVNET_FUNDED_KEY}" \
    "chars) — check that COMMUNITY_MNEMONIC is set in scripts/.env" >&2
  exit 1
fi

echo "== running devnet_tests pytest suite =="
# The two suites run separately so the live one's result can be checked on its
# own: `tests/` is offline and passes with no devnet at all, so a combined green
# exit code says nothing about the live probes having run.
poetry run pytest tests/

# `devnet_tests/` must be on the collection path for pytest to pick up
# devnet_tests/devnet_tests/conftest.py and its --devnet-config option.
LIVE_LOG="$(mktemp)"
poetry run pytest devnet_tests/ --devnet-config "$DEVNET_CONFIG" | tee "$LIVE_LOG"

# Every live probe pulls a fixture that skips itself when the devnet or the funded
# key is unusable, and an all-skipped run still exits 0 — same always-green
# outcome as an empty key above.
LIVE_PASSED="$(sed -n 's/.*[^0-9]\([0-9]\{1,\}\) passed.*/\1/p' "$LIVE_LOG" | tail -1)"
rm -f "$LIVE_LOG"
if [ -z "${LIVE_PASSED:-}" ] || [ "$LIVE_PASSED" -lt "$MIN_LIVE_PASSED" ]; then
  echo "only ${LIVE_PASSED:-0} live devnet tests passed, expected at least" \
    "$MIN_LIVE_PASSED — the live suite skipped itself instead of running" >&2
  exit 1
fi
SMOKE_SUCCEEDED=true
