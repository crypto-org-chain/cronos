#!/bin/bash
# Builds cronosd, brings up a one-validator devnet via pystarport, generates a
# devnet_tests config pointing at it, and runs the full devnet_tests pytest
# suite (unit + live) against it. Used by .github/workflows/devnet-smoke.yml
# and runnable locally the same way.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

BASE_PORT=26650
DATA_DIR="$(mktemp -d)"
DEVNET_CONFIG="$(mktemp)"

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

echo "== bringing up one-validator devnet in $DATA_DIR =="
pystarport init \
  --config scripts/cronos-single-devnet.yaml \
  --dotenv .env \
  --data "$DATA_DIR" \
  --base_port "$BASE_PORT" \
  --no_remove
setsid pystarport start --data "$DATA_DIR" --quiet &
PYSTARPORT_PID=$!

# pystarport 0.2.5's ports.py: evmrpc_port = base_port + 1, rpc_port = base_port + 7.
EVMRPC_PORT=$((BASE_PORT + 1))
RPC_PORT=$((BASE_PORT + 7))

wait_for_evmrpc_port() {
  for _ in $(seq 1 400); do
    if ! kill -0 "$PYSTARPORT_PID" 2>/dev/null; then
      echo "pystarport process died while waiting for the devnet to start" >&2
      exit 1
    fi
    (exec 3<>"/dev/tcp/127.0.0.1/$EVMRPC_PORT") 2>/dev/null && return 0
    sleep 0.1
  done
  return 1
}

echo "== waiting for EVM JSON-RPC on :$EVMRPC_PORT =="
if ! wait_for_evmrpc_port; then
  echo "devnet did not open its EVM JSON-RPC port within 40s" >&2
  exit 1
fi

cat > "$DEVNET_CONFIG" <<EOF
nodes:
  - name: cronos_777-1
    rpc: tcp://127.0.0.1:$RPC_PORT
    json_rpc: http://127.0.0.1:$EVMRPC_PORT
chain_id: 777
EOF

echo "== deriving funded key from COMMUNITY_MNEMONIC =="
# shellcheck disable=SC1091
source scripts/.env
export DEVNET_FUNDED_KEY
DEVNET_FUNDED_KEY="$(python3 -c '
import os
from eth_account import Account
Account.enable_unaudited_hdwallet_features()
print(Account.from_mnemonic(os.environ["COMMUNITY_MNEMONIC"]).key.hex())
')"

echo "== running devnet_tests pytest suite =="
cd devnet_tests
poetry install
poetry run pytest . --devnet-config "$DEVNET_CONFIG"
SMOKE_SUCCEEDED=true
