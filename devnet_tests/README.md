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
skip themselves (`_skip_rpc_diff_without_two_nodes` in `conftest.py`). Every
node must have a distinct `name`/`rpc`/`json_rpc`; the same endpoint listed
twice would make the cross-node diff compare a node to itself. `chain_id` is
checked against each node's `eth_chainId` when the fixture connects.

Most tests need a funded account to sign txs. Set its private key in the
`DEVNET_FUNDED_KEY` env var (deliberately not in the config file, so a live
devnet's key never ends up committed):

```
export DEVNET_FUNDED_KEY=<hex private key>
```

Tests needing it skip cleanly if it's unset.

## Run against a local devnet

`scripts/run-devnet-smoke.sh` (repo root) does this end to end: builds
`cronosd`, brings up a two-validator pystarport devnet from
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

# validator i listens on base_port + i*10; within a node, EVM JSON-RPC is
# base_port + 1 and CometBFT RPC is base_port + 7.
cat > /tmp/devnet-config.yaml <<EOF
nodes:
  - name: cronos_777-1-node0
    rpc: tcp://127.0.0.1:26657
    json_rpc: http://127.0.0.1:26651
  - name: cronos_777-1-node1
    rpc: tcp://127.0.0.1:26667
    json_rpc: http://127.0.0.1:26661
chain_id: 777
EOF

source scripts/.env
cd devnet_tests
export DEVNET_FUNDED_KEY=$(poetry run python3 -c '
import os
from eth_account import Account
Account.enable_unaudited_hdwallet_features()
print(Account.from_mnemonic(os.environ["COMMUNITY_MNEMONIC"]).key.hex())
')

poetry run pytest devnet_tests/ tests/ --devnet-config /tmp/devnet-config.yaml
```

`devnet_tests/` is the live suite, `tests/` the offline unit suite; naming both
runs everything the smoke script does. Don't collapse them to `.` — pytest only
registers `--devnet-config` when it collects
`devnet_tests/devnet_tests/conftest.py`, so the package directory has to be on
the collection path by name.

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
poetry run pytest devnet_tests/ --devnet-config /path/to/remote-config.yaml
```

## HTML report

`pytest-html` is a dev dependency. Generate a self-contained HTML report
(functional tests and the `rpc_diff` equivalence benchmark alike, since
`test_rpc_diff.py` runs it as a normal pytest test):

```
poetry run pytest devnet_tests/ --devnet-config /path/to/config.yaml \
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
writes it to a file. It exits non-zero on the same conditions
`test_rpc_diff.py` fails on: any mismatch, nothing compared, a method that
only ever errored, a method with no usable input at every sampled height, and
a range where no block called deployed bytecode. It has no HTML output of its
own — `--html` above is the report format for CI/local review.
