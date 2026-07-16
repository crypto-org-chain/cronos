# remote-benchmark

A standalone tx-load benchmark for Cronos, targeting one or more already-running
remote RPC endpoints. Ported from `testground/benchmark`, with all local/k8s
node-orchestration code (genesis generation, peer topology, container lifecycle)
dropped in favor of a config file listing the RPC endpoints to hit.

## Install

```bash
cd remote-benchmark
poetry install
```

## Config

All commands take `--config path/to/config.yaml` (JSON also supported via a
`.json` extension). See `sample-config.yaml`:

```yaml
endpoints:
  - name: primary
    rpc: https://rpc-t3.cronos.org
    json_rpc: https://evm-t3.cronos.org
  - name: secondary
    rpc: https://rpc2-t3.cronos.org
    json_rpc: https://evm2-t3.cronos.org
chain_id: 338
evm_denom: basetcro
gas_price: 5050000000000
global_seq: 999
tx_type: simple-transfer   # or erc20-transfer
msg_version: "1.3"
num_accounts: 2400
num_txs: 100
batch_size: 100
send_batch_size: 2000
send_interval: 0.2
telemetry: http://host:26660   # optional, enables block-stm/consensus stats
```

`endpoints` accepts one or many entries. When more than one is configured, tx
sending round-robins across all of them to spread load over the cluster.

Set `mode: eth` to target a plain Ethereum JSON-RPC node (no Cosmos/CometBFT
RPC) such as a local Anvil instance — see `sample-config-anvil.yaml` and
`scripts/anvil-smoke-test.sh`. In this mode `rpc`/`json_rpc` on an endpoint
should be the same JSON-RPC URL, and `stats`/`bench` report a trimmed set of
metrics (no mempool/Block-STM/consensus sections, since those need Cosmos SDK
telemetry).

## Usage

Account index 0 is reserved for the funding account (see `fund`); load-generating
accounts should start at index 1.

```bash
# fund the accounts that will be used to generate load
poetry run remote-benchmark fund --config sample-config.yaml 1 100

# check funded accounts
poetry run remote-benchmark check --config sample-config.yaml 1 100

# generate + send + report stats in one shot
poetry run remote-benchmark bench --config sample-config.yaml 1 100

# or step by step
poetry run remote-benchmark gen-txs --config sample-config.yaml 1 100 -o /tmp/txs.json
poetry run remote-benchmark send-txs --config sample-config.yaml /tmp/txs.json
poetry run remote-benchmark stats --config sample-config.yaml
```

## Test

```bash
poetry run pytest -vv
```

## Local Cronos devnet

`scripts/cronos-local-benchmark.sh <evm|batch>` launches the local 3-validator
Cronos devnet defined by `../integration_tests/configs/benchmark-3val.jsonnet`
(via `nix-shell` + `pystarport`, same as `../run.sh` /
`../docs/pystarport-3-validator-benchmark-setup.md`), funds a batch of test
accounts from the devnet's `community` account, then drives load against all
3 nodes round-robin and reports stats:

- `evm` — plain EVM JSON-RPC sends (`mode: eth`, `eth_sendRawTransaction`
  against each node's EVM JSON-RPC port).
- `batch` — Cosmos-wrapped `MsgEthereumTx` (`mode: cosmos`), batching
  multiple txs per Cosmos transaction (`batch_size: 5`) and broadcasting via
  CometBFT RPC.

The devnet and its data directory are torn down automatically on exit.
