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
sender_strategy: reuse       # or unique-per-tx
batch_size: 100
send_batch_size: 2000
send_interval: 0.2
telemetry: http://host:26660   # optional, enables block-stm/consensus stats
```

`endpoints` accepts one or many entries. When more than one is configured, tx
sending round-robins across all of them to spread load over the cluster.

### Resilient RPC access over SSH tunnels

Each endpoint can list extra tunnels to the same node via `rpc_pool`/
`json_rpc_pool`:

```yaml
endpoints:
  - name: remote-node0
    rpc: http://127.0.0.1:18657
    json_rpc: http://127.0.0.1:18545
    rpc_pool:
      - http://127.0.0.1:18658
      - http://127.0.0.1:18659
    json_rpc_pool:
      - http://127.0.0.1:18546
      - http://127.0.0.1:18547
```

Every call round-robins across an endpoint's `rpc`/`json_rpc` plus its pool
(not just on retry) — a single SSH-multiplexed connection head-of-line-blocks
every channel riding it on a hiccup, so spreading calls across independent
`ssh -L` processes means one wedged tunnel doesn't stall the whole run.
Sending load also round-robins across every endpoint's own pool combined
(cluster-wide), separate from the per-endpoint pool used for polling that one
node's status.

Pair this with `scripts/ssh-tunnel-supervisor.sh`, which spawns and
health-checks N independent tunnels to one remote host and recycles only the
slot that wedges:

```bash
scripts/ssh-tunnel-supervisor.sh ubuntu@<host> 26657:8545 \
  18657:18545 18658:18546 18659:18547
```

First arg is `user@host`. Second is `remote_rpc_port:remote_json_rpc_port`.
Remaining args are one `local_rpc:local_json_rpc` port pair per tunnel slot,
matching the pool ports in the config above.

### Running from a VM in the same cloud network as the devnet

If you have (or can spin up) a VM on the same cloud network as the devnet —
same VPC/subnet, not your laptop over SSH/VPN — skip tunnels entirely and hit
the devnet's private IP directly. This avoids both the VPN latency ramp-up
and the tunnel-wedging problem above:

```yaml
endpoints:
  - name: remote-node0
    rpc: http://<devnet-private-ip>:26657
    json_rpc: http://<devnet-private-ip>:8545
```

Get the repo onto that VM (`git clone`, checked out to the branch you're
testing), `poetry install`, then run the same `bench`/`soak` commands from
there. Confirm reachability first with `curl http://<devnet-private-ip>:26657/status`.

`sender_strategy: reuse` preserves the original workload: each of the
`num_accounts` accounts sends `num_txs` sequential transactions. Set
`sender_strategy: unique-per-tx` to give every generated transaction its own
sender at nonce 0. For native self-transfers, that makes each transaction touch
a disjoint account key and removes same-sender dependencies from BlockSTM. The
`fund` and `check` commands expand the requested logical account range
automatically; for example, accounts `1 100` with `num_txs: 10` fund/check
physical accounts `1..1000`.

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

# sustained load at a target rate for a fixed duration, with periodic checkpoints
poetry run remote-benchmark soak --config sample-config.yaml --nonce 0 --rate 500 --duration 300

# sweep a matrix of batch_size/num_accounts/etc, one bench run per cell
poetry run remote-benchmark sweep --config sample-config.yaml --results-dir /tmp/sweep

# diff two bench run records (delta table, config diff, optional HTML report)
poetry run remote-benchmark compare -o /tmp/compare.html run-a.json run-b.json

# RPC-only devnet preflight: mempool type + peer connectivity matrix
poetry run remote-benchmark preflight --config sample-config.yaml

# derive libp2p peer IDs + bootstrap_peers list for a set of nodes
poetry run remote-benchmark bootstrap-peers nodes.json
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
