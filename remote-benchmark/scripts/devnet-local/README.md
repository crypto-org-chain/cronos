# Local benchmark suite (pystarport devnet)

Spins up a local 1-, 3-, or 5-validator Cronos devnet on this machine, drives a
benchmark test case against it, writes an HTML report, and tears everything
down. Reproduces the
[V1.4 Benchmark wiki](https://github.com/crypto-org-chain/cronos/wiki/V1.4-Benchmark)
cases.

## Prerequisites

Nix + cachix (`cronos`, `dapp`), per
`../../../docs/pystarport-3-validator-benchmark-setup.md`, plus one
`poetry install` in `remote-benchmark/`.

## Run a test case

```bash
cd remote-benchmark/scripts/devnet-local
./run-benchmark.sh <1|3|5> <testcase>
```

Test cases: `simple-transfer`, `simple-transfer-unique`, `erc20-transfer`,
`batch-simple-transfer`, `batch-simple-transfer-unique`, `batch-erc20-transfer`.
The `-unique` variants are 1-validator only.

One invocation does all of this:

1. Initializes a fresh devnet in a temp dir and patches genesis with the
   predeployed contracts (ERC20, swap pool, NFT counter).
2. Starts the nodes, wires libp2p `bootstrap_peers` when enabled, and waits for
   every validator to see its peers.
3. Funds test accounts — count read straight from the chosen config's
   `num_accounts`.
4. Generates and sends the load, printing TPS/gas stats.
5. Writes a self-contained HTML report to
   `../../local/report/YYYYMMDD-HHMMSS.html`.
6. Tears the devnet down — temp dir removed, all `cronosd`/`pystarport`
   processes killed. `Ctrl-C` triggers the same cleanup.

The report opens with every benchmark parameter, then summary metrics and
block-level charts for tx count and EVM gas. Second-by-second charts show
committed TPS and gas throughput; the TPS view overlays a 5-second moving
average so sustained throughput reads apart from short block-time spikes.

Signed tx batches are cached under `../../local/.cache/genesis/` keyed by
config + binary hash, so repeat runs skip re-signing. The cache is only valid
against genesis-funded accounts starting at nonce 0 — delete it after changing
anything that shifts nonces.

### Environment variables

| Variable | Effect |
| --- | --- |
| `CRONOS_BIN=/path/to/cronosd` | Run against an already-built binary instead of building HEAD via nix. |
| `KEEP_DATA=1` | Leave the devnet data dir (node logs included) behind for post-mortem. |
| `MEMPOOL_MODE=auto\|legacy\|app` | Override the version-based mempool auto-detect. Default `auto`. |
| `MEMPOOL_SIZE=N` | Patch the CometBFT `mempool.size`. Only meaningful with the legacy-mempool config. |
| `SOAK_MODE=1` | Run `soak` instead of `bench`. Pass `--rate`/`--duration`/`--results` via `BENCH_EXTRA_ARGS`; no HTML report in this mode. |
| `BENCH_EXTRA_ARGS="..."` | Extra flags forwarded to `bench` (e.g. `--results`, `--repeat`, `--require-saturation`). |

```bash
CRONOS_BIN=/path/to/cronosd KEEP_DATA=1 ./run-benchmark.sh 5 simple-transfer
```

The script raises its own `nofile` ulimit to 65536, and warns if the shell's
hard limit clamps it below 8192 — at `send_batch_size: 8000` a stock ~1024
limit gives "too many open files" instead of the queuing the benchmark expects.

### Legacy-mempool fallback

`mempool.type: app` is v1.8.0-alpha+; older binaries panic on it. Binaries
below v1.8.0 automatically fall back to
`configs/benchmark-{1,3,5}val-legacy-mempool.jsonnet`, which drops the v1.8-only
CometBFT fields but keeps the app-level `PriorityNonceMempool`
(`app-config.mempool.max-txs`) so proposers still order by nonce/priority
instead of dropping to `NoOpMempool`.

These configs also get their `send_interval` raised at bench time. The classic
CometBFT mempool checks a tx's sequence against committed state only — it has
no pending-nonce tracking — so a nonce round sent before the previous one
commits is rejected, silently, since sends past the probe batch are
fire-and-forget.

## Run against an existing network

Same fund/check/bench/report workflow, arbitrary config:

```bash
./run-config-benchmark.sh --config ../../sample-config.yaml
```

The account range defaults to `1..num_accounts` from the config. Options:
`--start-account`, `--end-account`, `--nonce`, `--probe-batches`,
`--fund-batch-size`, `--fund-mode`, `--validators`, `--testcase`, `--output`,
`--skip-fund`, `--skip-check`. `--help` lists them all.

```bash
./run-config-benchmark.sh \
  --config ../../sample-config.yaml \
  --start-account 101 --end-account 200 \
  --nonce 0 \
  --output ../../local/report/testnet.html
```

Use `--skip-fund` for accounts funded in an earlier run, and `--skip-check`
when checking a large `unique-per-tx` account set would dominate setup time.
Reports land in `../../local/report/<config-name>-YYYYMMDD-HHMMSS.html` unless
`--output` says otherwise.

POSIX-shell compatible, so both invocation forms work:

```bash
sh run-config-benchmark.sh --config ../../sample-config-anvil.yaml
./run-config-benchmark.sh --config ../../sample-config-anvil.yaml
```

When an `eth`-mode endpoint identifies itself as Anvil, the script seeds the
deterministic funding account with enough balance for the selected range before
running `fund`. `--skip-fund` skips that too.

## Test cases

| Test case | Transaction | `batch_size` | Wiki case |
| --- | --- | --- | --- |
| `simple-transfer` | native transfer, one `MsgEthereumTx` per Cosmos tx | 1 | "Simple Transfer" |
| `simple-transfer-unique` | same count, one nonce-0 sender per transaction | 1 | BlockSTM conflict-free comparison |
| `erc20-transfer` | ERC20 `transfer()`, one per Cosmos tx | 1 | "ERC20 Transfer" |
| `batch-simple-transfer` | native transfer, 100 per Cosmos tx | 100 | "Batch Simple Transfer (100 size)" |
| `batch-simple-transfer-unique` | same, unique nonce-0 senders | 100 | BlockSTM comparison |
| `batch-erc20-transfer` | ERC20 `transfer()`, 100 per Cosmos tx | 100 | "Batch ERC20 Transfer (100 size)" |

The `-unique` variants keep the original workload size but expand their logical
accounts into one physical sender per transaction (e.g. 8,000 × 40 → 320,000
senders). `run-benchmark.sh` reserves and funds that expanded range
automatically. Compare against the non-unique case on the same machine to
isolate the effect of removing same-sender BlockSTM dependencies.

## Config files

**Devnet configs** — `configs/benchmark-{1,3,5}val.jsonnet` (plus
`-legacy-mempool` variants) carry the wiki's `config_patch`/`app_patch`/
`genesis_patch` tuning: `db_backend: rocksdb`, `async-check-tx`, Block-STM
executor with 32 workers, `memiavl` async commit. Mempool size follows the
wiki, and differs by validator count.

**Load configs** — `configs/{1,3,5}val-<testcase>.yaml`, one per validator-count
× test-case combination. `run-benchmark.sh` reads `num_accounts` straight from
the config to size the account range and the funder's balance, so changing it
there is enough — nothing else needs to stay in sync.

| Config | accounts × txs | Notes |
| --- | --- | --- |
| `1val-simple-transfer` | 8000 × 40 | matches the wiki exactly |
| `3val-simple-transfer` | 10000 × 10 | |
| `5val-simple-transfer` | 20000 × 15 | wiki lists no 5val options; `unique-per-tx` with `warmup_txs: 10` |
| `5val-batch-simple-transfer` | 8000 × 100, batch 100 | `commit_timeout: 600` — batched gas fills far more blocks |

**Contention configs** — `configs/1val-{erc20-transfer-hot,uniswap-swap,nft-mint,weighted-mix}.yaml`
exercise the Block-STM contention workloads (hot ERC20 recipient, swap pool
reserves, shared mint counter, and a weighted mix of all three). These are
**not** selectable through `run-benchmark.sh`; run them against an already-started
devnet via `./run-config-benchmark.sh --config configs/1val-nft-mint.yaml`.

`patch_erc20_genesis.py` injects every predeployed contract plus per-account
ERC20 balances into each node's `genesis.json`, between `pystarport init` and
`pystarport start`. Addresses are fixed and match what `transaction.py` targets.
It runs unconditionally — a harmless no-op for the native-transfer cases.

## Matrix drivers (scratch helpers)

`run_matrix.sh` and `run_combo.sh` drive the full binary × validator-count ×
test-case comparison, 3 rounds per combo, appending `overall_tps` to
`../../local/report/matrix-results.csv`. Binary paths are hardcoded — these are
local scratch tools, not part of the shipped suite.

## Port map (`base_port = 26650`)

Node `i`'s base is `base_port + i * 10`; each service is a fixed offset
(`p2p=+0`, `evmrpc=+1`, `evmrpc_ws=+2`, `grpc=+3`, `api=+4`, `pprof=+5`,
`grpc_tx_only=+6`, `rpc=+7`, `grpc_web=+8`).

| Node | RPC | EVM JSON-RPC | EVM WS |
| --- | --- | --- | --- |
| node0 | 26657 | 26651 | 26652 |
| node1 | 26667 | 26661 | 26662 |
| node2 | 26677 | 26671 | 26672 |
| node3 | 26687 | 26681 | 26682 |
| node4 | 26697 | 26691 | 26692 |

Nodes beyond node0 only exist at the matching validator count.

## Reading the output

`run-benchmark.sh` uses the `bench` command, which samples from the block
before submission through the block where every generated Cosmos transaction
has committed. It reports both the inner EVM transaction count and the Cosmos
envelope count, then prints `committed_cosmos_txs N/N`. It exits non-zero
rather than declaring a pass if the full workload doesn't commit within
`commit_timeout`.

`peak_tps` (highest single-block rate) is the number most comparable to the
wiki's figures. A low `overall_tps` next to a healthy `peak_tps` usually means
sending took longer than the chain needed to include everything — not that the
chain was the bottleneck.

### Wiki's own M4 MacBook Pro targets

| Test case | 1 validator | 3 validators |
| --- | --- | --- |
| simple-transfer | ~12088 | ~10236 |
| erc20-transfer | — | ~7417 |
| batch-simple-transfer | — | ~17200 |
| batch-erc20-transfer | — | ~9806 |

Coming in well below these? Tune `configs/*.yaml` first: `send_batch_size`
(concurrent broadcasts per interval), `send_interval` (pause between batches),
`send_conn_per_host`/`send_workers` (client-side concurrency ceiling — the
sender itself can be the bottleneck), and `num_txs`/`num_accounts` (total load).
CPU-side tuning (`block-stm-workers`, mempool size) lives in the jsonnet
configs.
