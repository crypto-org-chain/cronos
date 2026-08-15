# Cronos devnet TPS benchmark: v1.7.8 vs v1.8.0-alpha (antecache)

Local 5-validator devnet, `simple-transfer` load, following the methodology in
`Cronos EVM v1.8 Performance Benchmarking and Network Optimization.md` and the
mainnet-realistic settings from `validator-app.toml` / `validator-config.toml`.

Machine: Apple M1 Max, all 5 validators + load generator on one host.

## Binaries under test

- **v1.7.8** (true release binary) — legacy CometBFT mempool, no app-mempool support.
- **v1.8.0-alpha antecache build** (`cronos-antecache-build`, commit `7165e5ef`) — app-mempool
  (`mempool.type=app`), memiavl, block-stm.

These are two separate investigations, not a single before/after diff — v1.7.8 can't run
app-mempool at all, so its ceiling is set by the legacy mempool; v1.8-alpha's ceiling is set by
gas limit and execution/storage tuning instead.

## Headline numbers

| Binary | Config | median_tps | Bottleneck |
|---|---|---|---|
| v1.7.8 | legacy mempool, max_gas=105M, mainnet mempool.size=2000 | **1083.70** (Run 1) / 1279.37 (Run S3, 2x load) | CometBFT mempool.size=2000 cap — repeatedly full, load generator retry-storms. Gas util only 8-12%: nowhere near the execution/gas ceiling. |
| v1.8.0-alpha antecache | app-mempool, max_gas=210M, mainnet consensus timeouts, memiavl cache-size=1000 | **~4623.7 avg / 4719.78 best single-run** (Phase 3, 3 independent runs) | Single-machine execution/consensus cadence wall — bigger blocks take proportionally longer to gossip+commit than the extra gas is worth. Gas util plateaus ~53% before blocktime blows up. |

v1.8-alpha's app-mempool removes v1.7.8's binding constraint entirely (no fixed-size CometBFT
mempool to overflow) and reaches **~4.3x** v1.7.8's median TPS on this hardware.

## Phase 1: gas-limit sweep (v1.8-alpha antecache, app-mempool)

| max_gas | median_tps | gas util | blocktime (median) | failed_txs | verdict |
|---|---|---|---|---|---|
| 105M (Run 2) | 1711.39 | 11.1% | 517ms | 23.3% | baseline, not saturated |
| 210M (Run 3b) | 4456.22 | 40.7% | 951ms | 28.0% | +160% tps, still not saturated — continue |
| 400M (Run 4) | 4199.64 | 47.8% | 2210ms (+132%) | 25.8% | flat/negative tps despite 2x gas — **abandon sweep, blocktime wall** |

**Phase-1 max: max_gas=210M.** Bigger gas ceilings don't help once block-execution/gossip/commit
time dominates; 400M made blocks slower without moving the needle on throughput.

## Phase 2: sequential tuning layers (on top of the 210M baseline)

Kept a change only if it improved median_tps ≥3% with gates (blocktime, gas util, failed_txs)
still holding.

| Layer | Change | Δ median_tps | Verdict |
|---|---|---|---|
| 0a (execution) | block-stm-pre-estimate: false→true | +0.008% | Revert — flat, blocktime worse |
| 0b (execution) | block-stm-workers: 0→4 | +1.37% | Revert — below 3% bar, blocktime worse (CPU contention w/ 5 validators on one host) |
| 1a (storage) | memiavl cache-size: 0→1000 (mainnet default) | **+5.9%** | **Keep** |
| 1b (storage) | memiavl async-commit-buffer: 16→32 | −4.4% | Revert |
| 2 (consensus) | aggressive prevote/precommit/commit timeouts | −2.7% | Revert — faster rounds, more round-skip churn, net loss |
| 3a (mempool) | reap_interval: 500ms→200ms | −11.6% | Revert — worst single-knob regression; smaller/more-frequent blocks add per-block overhead |
| 3b (mempool) | tx-cache-size: 100000→200000 | −4.4% | Revert — cache wasn't the bottleneck |

**Phase-2 final config = Phase-1 baseline + memiavl cache-size=1000.** Every other tested knob
(execution parallelism, storage commit buffering, consensus timeout aggressiveness, mempool reap
cadence/cache size) either was flat or regressed on this single-machine setup — consistent with
CPU/execution contention (5 validators + load generator sharing one host), not any individual knob,
being the dominant constraint at this scale.

## Phase 3: stability confirmation

- **Repeat-stability:** 3 independent runs of the final Phase-2 config: median_tps 4494.87 /
  4656.47 / 4719.78 (4.8% spread, mean 4623.7). Stable, not a one-off high-variance measurement.
- **Soak (600s @ ~3930 tx/s target):** all evaluated gates passed — tps_trend +1.01/s (rising, no
  degradation), block-time trend flat, tps_floor passed. No leak/degradation signal over 10
  minutes of continuous load.
- **Machine-drift re-run of the v1.7.8 baseline:** qualitatively matches Run 1 (0% failed_txs,
  low gas util, mempool.size=2000-bound) — no drift evidence, though not a byte-for-byte
  comparison since num_accounts was widened 10000→20000 mid-Phase-1 for the whole sweep and never
  reverted.

## Caveats

- **R1 — app-mempool mempool-status gate is uninformative.** `/num_unconfirmed_txs` (what
  `--require-saturation` polls) reads 0 the entire run under `mempool.type=app`, regardless of
  actual pending load. Every app-mempool run in this report was gated manually on gas
  utilization + blocktime trend + failed-tx rate instead of `--require-saturation`.
- **R2 — failed_txs sits at 15-29% across every app-mempool cell.** Root cause: the load
  generator reuses a fixed pool of sender accounts and round-robins broadcast across all 5
  endpoints; under app-mempool's pending-nonce tracking this produces genuine nonce-conflict/
  stale-tx rejections (ABCI code != 0, counted from block_results) at load. This is a load-
  generator artifact of this harness, not a mainnet nonce-handling defect — but it means
  committed_cosmos_txs undercounts total send attempts. median_tps as reported only counts
  committed txs, so the headline numbers are unaffected; there's just wasted retry traffic behind
  them.
- **R3 — single-machine execution contention.** All 5 validators plus the load generator run on
  one M1 Max. Every Phase-2 execution/consensus knob that added concurrency (extra block-stm
  workers, faster consensus rounds) made things worse here, most likely because it competes with
  the other validator processes for the same cores rather than because the knob is bad in
  general. On real multi-host mainnet-scale hardware these knobs may behave differently — this
  report's Phase-2 conclusions are specific to the single-machine devnet setup.
- **`--repeat`'s in-run reuse is broken for app-mempool** — reusing the same sender accounts
  across repeats hits permanent nonce gaps from the R2 failure phenomenon (not a transient
  race). A defensive settle-retry was added to `remote_benchmark/runner.py`'s
  `current_sender_nonce` (helps genuinely transient cases, tests updated) but doesn't close this
  gap; Phase 3's repeat-stability check used independent runs instead.
- **v1.7.8 vs v1.8-alpha is not an apples-to-apples "improvement %."** They hit different walls
  (mempool-size cap vs execution/consensus cadence) — the ~4.3x gap is the effect of app-mempool
  removing v1.7.8's specific ceiling, not a general "v1.8 is 4.3x faster" claim that would hold
  under every workload/config.

## Benchmark-tooling changes made during this investigation

- `remote_benchmark/runner.py`: `current_sender_nonce` now retries its post-run nonce scan (5x,
  2s apart) before raising, to tolerate genuinely transient in-flight-tx settle delays. Tests
  updated (`tests/test_runner.py`) to mock `time.sleep` and cover both the persistent-divergence
  and settles-on-retry paths.
- `scripts/devnet-local/run-benchmark.sh`: added `SOAK_MODE=1` to route the script's existing
  devnet-standup plumbing through `remote-benchmark soak` instead of `bench`, so the `soak`
  CLI command (pre-existing, previously unreachable from this script) can run against a properly
  initialized local devnet.

## Raw data

Per-run logs, JSON result records, and devnet data-dir paths for every run cited above are in
`/tmp/bench/NOTES.md` and `/tmp/bench/*.json` / `*.log`.
