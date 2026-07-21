# Local benchmark suite (M4 MacBook Pro / pystarport)

Everything needed to spin up a local 1- or 3-validator Cronos devnet on this
machine and reproduce the [V1.4 Benchmark wiki](https://github.com/crypto-org-chain/cronos/wiki/V1.4-Benchmark)
test cases, grouped in this one folder.

## Prerequisites

Same as `../../docs/pystarport-3-validator-benchmark-setup.md`: Nix +
cachix (`cronos`, `dapp`) installed, and `poetry install` run once in
`remote-benchmark/`.

## Usage

```bash
cd remote-benchmark/local
./run-benchmark.sh <1|3> <simple-transfer|simple-transfer-unique|erc20-transfer|batch-simple-transfer|batch-erc20-transfer>
```

This initializes a fresh devnet under a temp data dir, patches its genesis
with a predeployed ERC20 contract, starts it, funds test accounts (count
read from the chosen config's `num_accounts`, matching the wiki), generates
and sends load, prints TPS/gas stats, writes a timestamped, self-contained HTML
report to `report/YYYYMMDD-HHMMSS.html`, then tears the devnet down (temp dir
removed, all `cronosd`/`pystarport` processes killed) on exit — `Ctrl-C` at
any point triggers the same cleanup. The report starts with every benchmark
parameter and includes summary metrics plus block-level charts for transaction
count and EVM gas consumed. Second-by-second charts show committed TPS and gas
throughput; the TPS view overlays a 5-second moving average to make sustained
throughput easier to distinguish from short block-time spikes.

To run the same fund/check/bench/report workflow against an already-running
network described by an arbitrary config file, use:

```bash
./run-config-benchmark.sh --config ../sample-config.yaml
```

The account range defaults to `1..num_accounts` from the config. It can be
overridden, along with benchmark and report options:

```bash
./run-config-benchmark.sh \
  --config ../sample-config.yaml \
  --start-account 101 \
  --end-account 200 \
  --nonce 0 \
  --output report/testnet.html
```

By default the script funds and checks the selected accounts before running
the benchmark. Use `--skip-fund` for accounts funded in an earlier run and
`--skip-check` when checking a large `unique-per-tx` account set would dominate
setup time. Run `./run-config-benchmark.sh --help` for all options. Reports are
written to `report/<config-name>-YYYYMMDD-HHMMSS.html` unless `--output` is
provided. The script is POSIX-shell compatible, so both invocation forms are
supported:

```bash
sh run-config-benchmark.sh --config ../sample-config-anvil.yaml
./run-config-benchmark.sh --config ../sample-config-anvil.yaml
```

When an Ethereum-mode endpoint identifies itself as Anvil, the script seeds the
deterministic funding account with the balance required for the selected account
range before running `fund`; `--skip-fund` also skips this preparation.

## What each test case does

| Test case | tx | batch_size | Matches wiki |
| --- | --- | --- | --- |
| `simple-transfer` | plain native transfer, one `MsgEthereumTx` per cosmos tx | 1 | "Simple Transfer" |
| `simple-transfer-unique` | same native-transfer count, one nonce-0 sender per transaction | 1 | BlockSTM conflict-free comparison |
| `erc20-transfer` | ERC20 `transfer()` call, one `MsgEthereumTx` per cosmos tx | 1 | "ERC20 Transfer" |
| `batch-simple-transfer` | native transfer, 100 `MsgEthereumTx` per cosmos tx | 100 | "Batch Simple Transfer (100 size)" |
| `batch-erc20-transfer` | ERC20 `transfer()` call, 100 `MsgEthereumTx` per cosmos tx | 100 | "Batch ERC20 Transfer (100 size)" |

The `erc20-transfer`/`batch-erc20-transfer` cases need every sending account
to already hold an ERC20 balance — `patch_erc20_genesis.py` injects the
contract (fixed address `remote_benchmark.erc20.CONTRACT_ADDRESS`, matching
what `transaction.py`'s `erc20_transfer_tx` targets) plus balances for
account indices `1..num_accounts` into every node's `genesis.json`, right
after `pystarport init` and before `pystarport start`. It's run
unconditionally (harmless no-op cost for the simple-transfer cases).

## Config files

- `configs/benchmark-1val.jsonnet` / `benchmark-3val.jsonnet` — pystarport
  configs with the wiki's `config_patch`/`app_patch`/`genesis_patch`
  benchmark tuning (`db_backend: rocksdb`, `async-check-tx`, block-STM
  executor with 32 workers, `memiavl` async commit, etc). Mempool size
  differs per the wiki (50000 for 1 validator, 100000 for 3).
- `configs/{1,3}val-<testcase>.yaml` — `remote-benchmark` configs (one per
  validator-count × test-case combination). `num_accounts`/`num_txs`/
  `batch_size` match the wiki's options exactly per scenario (e.g. 8000
  accounts × 40 txs for 1-validator simple-transfer, 2400 accounts × 100
  txs/batch for the batch cases). `run-benchmark.sh` reads `num_accounts`
  straight from the config to size `END_ACCOUNT` and the funder's balance,
  so changing it here is enough — no other file needs to stay in sync.

The one-validator `simple-transfer-unique` config keeps the original workload
size (320,000 transfers), but expands its 8,000 logical accounts into 320,000
physical senders. `run-benchmark.sh` reserves and funds that expanded range
automatically. Compare it with `simple-transfer` on the same machine to isolate
the effect of removing same-sender BlockSTM dependencies.

## Port map (`base_port = 26650`)

Node `i`'s base is `base_port + i * 10`; each service is a fixed offset from
that (`p2p=+0`, `evmrpc=+1`, `evmrpc_ws=+2`, `grpc=+3`, `api=+4`, `pprof=+5`,
`grpc_tx_only=+6`, `rpc=+7`, `grpc_web=+8`).

| Node | RPC (tcp) | EVM JSON-RPC | EVM WS |
| ---- | --------- | ------------ | ------ |
| node0 | 26657 | 26651 | 26652 |
| node1 | 26667 | 26661 | 26662 |
| node2 | 26677 | 26671 | 26672 |

(node1/node2 only exist for the 3-validator devnet.)

## TPS targets (wiki's own M4 MacBook Pro numbers)

| Test case | 1 validator | 3 validators |
| --- | --- | --- |
| simple-transfer | ~12088 | ~10236 |
| erc20-transfer | — | ~7417 |
| batch-simple-transfer | — | ~17200 |
| batch-erc20-transfer | — | ~9806 |

If a run comes in noticeably below these, the first things to tune are in
`configs/*.yaml`: `send_batch_size` (concurrent broadcasts per interval),
`send_interval` (pause between broadcast batches — too short floods
`CheckTx` and causes consensus round timeouts, too long leaves throughput on
the table), and `num_txs`/`num_accounts` (total load size). CPU-side tuning
(`block-stm-workers`, mempool size) lives in the jsonnet configs.

**Reading `bench`'s output**: `run-benchmark.sh` uses the CLI's `bench`
command, which samples from the block immediately before submission through
the block where every generated Cosmos transaction has committed. It reports
both the generated inner EVM transaction count and the Cosmos envelope count,
then prints `committed_cosmos_txs N/N`. The command exits nonzero instead of
declaring the benchmark passed if the complete workload does not commit within
120 seconds after sending finishes. `peak_tps` (the highest single-block rate
seen) is the number most comparable to the wiki's own figures; a low
`overall_tps` next to a healthy `peak_tps` usually means the chain kept up fine
but sending took longer than the chain needed to include everything, not that
the chain itself is the bottleneck.
