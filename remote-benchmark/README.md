# remote-benchmark

Tx-load benchmark for Cronos. Points at one or more already-running RPC
endpoints listed in a config file, generates signed transactions, sends them,
and reports TPS/gas/block-time statistics.

Ported from `testground/benchmark`, with all node-orchestration code (genesis
generation, peer topology, container lifecycle) dropped.

- Want a local devnet benchmarked end to end? Use
  [`scripts/devnet-local/`](scripts/devnet-local/README.md) — it spins the
  devnet up, funds accounts, runs the load, and writes an HTML report.
- Want to hit an existing network? Write a config (below) and run `bench`.

## Install

```bash
cd remote-benchmark
poetry install
```

## Quick start

```bash
# account index 0 is the funder; load accounts start at 1
poetry run remote-benchmark fund  --config sample-config.yaml 1 100
poetry run remote-benchmark check --config sample-config.yaml 1 100
poetry run remote-benchmark bench --config sample-config.yaml 1 100
```

`bench` samples from the block before the first send through the block where
every generated transaction has committed, then prints the stats. It exits
non-zero if the full workload never commits within `commit_timeout`.

## Commands

Every command takes `--config path/to/config.yaml` (`.json` also works).
`START END` are inclusive account indices.

| Command | Signature | What it does |
| --- | --- | --- |
| `fund` | `fund [--batch-size 200] [--mode cosmos\|eth] START END` | Move funds from account 0 into the load accounts. |
| `check` | `check START END` | Print each load account's balance and nonce. |
| `bench` | `bench [options] START END` | Generate + send + measure in one shot. See below. |
| `soak` | `soak --rate TPS --duration S [--checkpoint-interval 30] [--results PATH] START END` | Open-loop sustained load. Trend-fits RSS/TPS/block time for a leak/degradation verdict. |
| `sweep` | `sweep --results-dir DIR [--no-stop-on-degradation] MATRIX START END` | One `bench` run per matrix cell. Stops at the first cell failing the saturation gates unless told otherwise. |
| `gen-txs` | `gen-txs [--nonce 0] [--start-account 0] [-o FILE] START END` | Sign a batch and write it out (default stdout). |
| `send-txs` | `send-txs [--sync\|--async] FILE` | Send a previously generated batch. |
| `stats` | `stats [--count 30]` | Stats for the last `--count` blocks, no load sent. |
| `compare` | `compare [-o out.html] A.json B.json` | Delta table + config diff between two run records. |
| `preflight` | `preflight` | RPC-only devnet check: resolved `mempool.type` per node + peer connectivity matrix. Exits non-zero on unreachable nodes, missing peer links, or nodes disagreeing on mempool type. |
| `bootstrap-peers` | `bootstrap-peers [--port 26656] [-o FILE] NODES.json` | Derive libp2p peer IDs and a `bootstrap_peers` list. |

### `bench` options

| Option | Default | Purpose |
| --- | --- | --- |
| `--nonce N` | query the chain | Starting nonce. Pass it explicitly when the chain's answer may be stale. |
| `--probe-batches N` | 1 | Send this many leading batches synchronously so `CheckTx` rejections surface immediately instead of vanishing into `broadcast_tx_async`. 0 disables. |
| `--results PATH` | — | Write a run record: config snapshot, node fingerprint, per-block series, summary metrics, saturation verdict. |
| `--repeat N` | 1 | Run the same load N times. Per-run records go to `<stem>-runN<suffix>`; the aggregate goes to `--results`. |
| `--require-saturation` | off | Exit non-zero with reasons when the saturation gates (gas utilization, mempool pending, failed-tx rate) aren't met. |
| `--txs-cache PATH` | — | Reuse a signed batch across runs (write it if absent). Valid **only** when the accounts start at nonce 0 every run — a stale cache replayed at a different nonce fails `CheckTx`. |

### `sweep` matrix file

JSON or YAML:

```yaml
apply_config_hook: ./restart-devnet.sh   # runs per cell with the cell's values
restart_wait_s: 10
axes:
  send_batch_size: [2000, 8000]
  num_accounts: [1000, 8000]
```

## Config

```yaml
endpoints:
  - name: node0
    rpc: http://127.0.0.1:26657        # CometBFT RPC
    json_rpc: http://127.0.0.1:26651   # EVM JSON-RPC
chain_id: 777
evm_denom: basetcro
gas_price: 5000000000000
global_seq: 0
tx_type: simple-transfer
msg_version: "1.4"
num_accounts: 8000
num_txs: 40
sender_strategy: reuse
batch_size: 1
send_batch_size: 8000
send_interval: 0.05
commit_timeout: 120
telemetry: http://127.0.0.1:26660      # optional: enables Block-STM/consensus stats
```

### Fields

**Endpoints**

| Field | Default | Meaning |
| --- | --- | --- |
| `endpoints` | required | One or more nodes. Load sending round-robins across all of them; polling uses the first (`primary`). |
| `endpoints[].rpc_pool` / `json_rpc_pool` | `[]` | Extra URLs to the *same* node. See [SSH tunnels](#resilient-rpc-access-over-ssh-tunnels). |
| `endpoints[].node_config` | `{}` | Operator-declared settings not observable over RPC (`mempool.type`, libp2p on/off, Block-STM workers). Recorded verbatim into the run record's node fingerprint. |
| `endpoints[].node_exporter` | — | node_exporter base URL for that node's host, for disk/network I/O stats. Per-endpoint because it's host-level, unlike the single global `telemetry`. |
| `telemetry` | — | Prometheus endpoint. Without it, Block-STM and consensus sections are omitted. |

**Workload shape**

| Field | Default | Meaning |
| --- | --- | --- |
| `mode` | `cosmos` | `cosmos` wraps txs in `MsgEthereumTx` and broadcasts via CometBFT RPC. `eth` sends raw `eth_sendRawTransaction`. |
| `tx_type` | `simple-transfer` | See [workload types](#workload-types). |
| `mix_weights` | — | Required when `tx_type: weighted-mix`. `{tx_type: weight}`, weights non-negative and summing above 0. |
| `msg_version` | `"1.4"` | Ethermint `MsgEthereumTx` encoding version. |
| `num_accounts` | 100 | Logical sending accounts. |
| `num_txs` | 1 | Transactions per logical account. |
| `sender_strategy` | `reuse` | See [sender strategies](#sender-strategies). |
| `batch_size` | 1 | Inner `MsgEthereumTx` count per Cosmos transaction. >1 is the "batch" workload. |
| `global_seq` | 0 | Account-derivation seed. Change it to get a disjoint account set. |

**Send pacing**

| Field | Default | Meaning |
| --- | --- | --- |
| `send_batch_size` | 500 | Concurrent broadcasts per interval. |
| `send_interval` | 0.5 | Seconds between broadcast batches. Too short floods `CheckTx` and trips consensus round timeouts; too long leaves throughput unused. |
| `send_conn_per_host` | 200 | Per-host HTTP connection cap. 200 protects a tunneled `ssh -L` endpoint from a connection burst; raise it for direct-loopback endpoints. Applies **per sender process**, so a host actually sees `send_workers × send_conn_per_host`. |
| `send_workers` | 1 | >1 fans sending across OS processes. One event loop is CPU-bound on JSON-RPC serialization, so raising `send_conn_per_host` alone can't get past that wall. Only `bench` honors this — `send-txs` and `soak` always send from one process. |
| `warmup_txs` | 0 | Throwaway txs per account before the measured load, to pay mempool/JIT/connection-pool warm-up costs outside the measured window. Not supported under `unique-per-tx` (skipped, with a message on stderr). |

**Measurement**

| Field | Default | Meaning |
| --- | --- | --- |
| `commit_timeout` | 120 | Seconds to keep waiting for commits after the last send. Batched workloads need more: ~100× more gas per Cosmos tx fills far more blocks, each gas-saturated. |

## Workload types

| `tx_type` | What each transaction does | Needs predeployed contract |
| --- | --- | --- |
| `simple-transfer` | Native self-transfer. | no |
| `erc20-transfer` | `transfer()` on the benchmark ERC20; every sender needs its own balance. | yes |
| `erc20-transfer-hot` | ERC20 transfers all targeting one recipient — maximum storage contention for Block-STM. | yes |
| `uniswap-swap` | Swap against a fixed-reserve pool. Contended reserves. | yes |
| `nft-mint` | Mint from a shared counter. Contended single slot. | yes |
| `weighted-mix` | Dispatches to the above by `mix_weights`. | as per mix |

The contract-backed types need those contracts in genesis.
`scripts/devnet-local/patch_erc20_genesis.py` injects all of them (ERC20 + pool
+ NFT counter) at fixed addresses matching what `transaction.py` targets.

## Sender strategies

`reuse` (default) — each of the `num_accounts` accounts sends `num_txs`
sequential nonces. Same-sender ordering creates BlockSTM dependencies.

`unique-per-tx` — every generated transaction gets its own sender at nonce 0.
For native self-transfers each transaction then touches a disjoint account key,
removing same-sender dependencies from BlockSTM. `fund` and `check` expand the
requested logical range automatically: accounts `1 100` with `num_txs: 10` fund
and check physical accounts `1..1000`.

## Ethereum-only mode

Set `mode: eth` to target a plain Ethereum JSON-RPC node (e.g. Anvil) with no
Cosmos/CometBFT RPC. Set `rpc` and `json_rpc` to the same URL. `stats` and
`bench` then report a trimmed metric set — no mempool, Block-STM, or consensus
sections, since those need Cosmos SDK telemetry. See `sample-config-anvil.yaml`
and `scripts/anvil-smoke-test.sh`.

## Reaching a remote devnet

### Running from a VM in the same network (preferred)

If you can put a VM on the devnet's own VPC/subnet, skip tunnels and hit
private IPs directly. This avoids both the VPN latency ramp and the
tunnel-wedging problem below.

```yaml
endpoints:
  - name: remote-node0
    rpc: http://<devnet-private-ip>:26657
    json_rpc: http://<devnet-private-ip>:8545
```

Clone the repo onto that VM, `poetry install`, run the same commands. Confirm
reachability first: `curl http://<devnet-private-ip>:26657/status`.

### Resilient RPC access over SSH tunnels

One SSH-multiplexed connection head-of-line-blocks every channel riding it on a
hiccup. List extra independent tunnels to the same node so calls spread across
separate `ssh -L` processes:

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

Every call round-robins across `rpc`/`json_rpc` plus the pool — always, not
only on retry — so one wedged tunnel can't stall the run. Load sending
round-robins across every endpoint's pools combined (cluster-wide), separate
from the per-endpoint pool used to poll one node's status.

`scripts/ssh-tunnel-supervisor.sh` spawns and health-checks N independent
tunnels and recycles only the slot that wedges:

```bash
scripts/ssh-tunnel-supervisor.sh ubuntu@<host> 26657:8545 \
  18657:18545 18658:18546 18659:18547
```

First arg `user@host`. Second `remote_rpc_port:remote_json_rpc_port`. Remaining
args are one `local_rpc:local_json_rpc` pair per tunnel slot, matching the pool
ports above.

## Test

```bash
poetry run pytest -q
```

## Local devnet

`scripts/devnet-local/` is the full local suite (1/3/5 validators, HTML
reports, tx caching, legacy-mempool fallback) — see
[its README](scripts/devnet-local/README.md).

`scripts/cronos-local-benchmark.sh <evm|batch>` is the older, smaller
alternative: a 3-validator devnet from
`../integration_tests/configs/benchmark-3val.jsonnet`, funded from the
`community` account, load driven round-robin across all 3 nodes. `evm` sends
plain EVM JSON-RPC; `batch` sends Cosmos-wrapped `MsgEthereumTx` at
`batch_size: 5`. Devnet and data dir are torn down on exit.
