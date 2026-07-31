# devnet_tests

Functional/behavioral test suite that runs against an **already-running**
Cronos devnet — local (pystarport) or remote. It doesn't launch a chain
itself; you point it at one via `--devnet-config`.

## Install

```
cd devnet_tests
poetry install
```

## Devnet config format

A YAML file listing the node(s) to test against:

```yaml
nodes:
  - name: cronos_777-1
    rpc: tcp://127.0.0.1:26657       # CometBFT RPC
    json_rpc: http://127.0.0.1:8545  # EVM JSON-RPC
chain_id: 777
```

List 2+ nodes to also enable the cross-node tests (`test_rpc_diff.py`,
`test_state_safety.py::test_app_hash_agreement`) — with only 1 node they
skip themselves (`_skip_rpc_diff_without_two_nodes` in `conftest.py`).

Most tests need a funded account to sign txs. Set its private key in the
`DEVNET_FUNDED_KEY` env var (deliberately not in the config file, so a live
devnet's key never ends up committed):

```
export DEVNET_FUNDED_KEY=<hex private key>
```

Tests needing it skip cleanly if it's unset.

## Run against a local devnet

`scripts/run-devnet-smoke.sh` (repo root) does this end to end: builds
`cronosd`, brings up a one-validator pystarport devnet from
`scripts/cronos-single-devnet.yaml`, derives the funded key from
`COMMUNITY_MNEMONIC`, writes a matching devnet config, and runs the suite.
Run it directly:

```
./scripts/run-devnet-smoke.sh
```

To iterate manually instead (e.g. to keep the devnet up between runs):

```
make build LEDGER_ENABLED=false
export PATH="$PWD/build:$PATH"

pystarport init --config scripts/cronos-single-devnet.yaml \
  --dotenv .env --data /tmp/devnet-data --base_port 26650 --no_remove
pystarport start --data /tmp/devnet-data &

cat > /tmp/devnet-config.yaml <<EOF
nodes:
  - name: cronos_777-1
    rpc: tcp://127.0.0.1:26657
    json_rpc: http://127.0.0.1:8545
chain_id: 777
EOF

source scripts/.env
export DEVNET_FUNDED_KEY=$(python3 -c '
import os
from eth_account import Account
Account.enable_unaudited_hdwallet_features()
print(Account.from_mnemonic(os.environ["COMMUNITY_MNEMONIC"]).key.hex())
')

cd devnet_tests
poetry run pytest . --devnet-config /tmp/devnet-config.yaml
```

If you're testing a `vendor/` patch, build with `-mod=vendor` explicitly —
`make build` passes `-mod=readonly`, which disables Go's automatic
vendor-mode detection and silently resolves deps from the module cache
instead of `vendor/`:

```
go build -mod=vendor -tags "netgo objstore pebbledb mainnet" -o build/cronosd ./cmd/cronosd
```

## Run against a remote devnet

Same suite, just point the config at the remote endpoints and export the
funded key for that network:

```yaml
nodes:
  - name: remote-node-0
    rpc: tcp://devnet.example.internal:26657
    json_rpc: http://devnet.example.internal:8545
chain_id: 777
```

```
export DEVNET_FUNDED_KEY=<hex private key funded on that devnet>
cd devnet_tests
poetry run pytest . --devnet-config /path/to/remote-config.yaml
```

## HTML report

`pytest-html` is a dev dependency. Generate a self-contained HTML report
(functional tests and the `rpc_diff` equivalence benchmark alike, since
`test_rpc_diff.py` runs it as a normal pytest test):

```
poetry run pytest . --devnet-config /path/to/config.yaml \
  --html=report.html --self-contained-html
```

Open `report.html` — pass/fail per test, tracebacks on failure, and
durations. `-v` on the pytest invocation gives more detail in the report's
captured log section.

For the raw `rpc_diff` equivalence numbers (per-method match/mismatch
counts) rather than a pass/fail report, use its CLI directly:

```
poetry run devnet-tests rpc-diff --config /path/to/config.yaml \
  --start <height> --end <height> --out rpc-diff.json
```

This prints the JSON report and equivalence rate to stdout; `--out` also
writes it to a file. It has no HTML output of its own — `--html` above is
the report format for CI/local review.
