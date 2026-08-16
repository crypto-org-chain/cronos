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

The v1.8-alpha number above is not updated for R4/Phase 7, and nothing found in either raises it.
Feeding the chain harder made throughput *worse*, not better (see R4), and Phase 7's
floor-compression knobs only move the idle-block cadence — under load they left median_blocktime and
median_tps inside the noise band. Treat 4623.7 as the number for this report's tested config space,
not a hard ceiling this hardware can't beat.

**The strongest result in this report is a negative one:** throughput on this setup peaks with blocks
only ~45% full, and saturating them (92% gas utilization, achieved via multi-process senders in R4)
*halves* median_tps while tripling blocktime. This chain congestion-collapses under full load rather
than plateauing, which reframes several earlier phases — see R4 and Phase 7.

**Phases 1, 2, 3, and 6 below were all measured against a client capped at ~4300-4900 tx/s of
send rate (see caveat R4) — none of them ever filled a block.** Phase 1 in particular called
`max_gas=400M` a "blocktime wall" at only 47.8% gas utilization; the gas ceiling was never the
binding constraint there, the client's send rate was. These phases were not re-measured against an
uncapped client (out of scope for the R4 fix-up); treat their specific numbers as suspect and their
qualitative verdicts (this hardware's execution/consensus contention dominates over these
particular knobs) as still informative.

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

## Phase 4: v1.7.8 `mempool.size` sweep (legacy CometBFT mempool)

Headline v1.7.8 number (1083.70-1279.37 median_tps) is bound by the mainnet-realistic
`mempool.size=2000` cap. Swept the cap upward on the same 5-validator/`simple-transfer` setup to
see whether the ceiling simply moves, or whether some other constraint takes over.

| mempool.size | median_tps | overall_tps | committed/300000 | send-rejected (never reached mempool) | failed_txs (ABCI, post-commit) | verdict |
|---|---|---|---|---|---|---|
| 2000 (baseline) | 1245.90 | 1191.11 | 226065 (stalled — mempool repeatedly full) | 66747 (22.2%) | 0 (0.0%) | matches Run 1 baseline |
| 10000 | **2047.42** | 2048.50 | 296218 | 3399 (1.1%) | 0 (0.0%) | best of the sweep — +64% over baseline |
| 50000 | 1549.62 | 1663.91 | 298062 | 1508 (0.5%) | 0 (0.0%) | worse than 10000 despite fewer rejections |
| 100000 | 1499.46 | 1244.33 | 299802 | 157 (0.05%) | 0 (0.0%) | worse still — most commits, lowest tps |

**Not monotonic.** Bigger cap does buy more sends through (commit counts climb: 226k → 296k →
298k → 300k), but the *rate* peaks at 10000 and falls off past it — a bigger mempool costs more
gossip/recheck work per block than it saves in rejected sends. Recommend raising v1.7.8's default
from 2000 to ~10000, not removing the cap.

**v1.7.8's failure mode is send-side, not execution-side.** Unlike v1.8-alpha's app-mempool (R2:
15-29% *post-commit* ABCI failures from nonce conflicts), v1.7.8's legacy mempool rejects txs
before they ever enter CheckTx — the sender's retry logic exhausts and gives up client-side once
the mempool is full. `failed_txs` (ABCI code != 0, counted from `/block_results`) stays 0% across
every cap tested; all the loss shows up as "never reached the mempool" instead, and that rate falls
straight from 22.2% to 0.05% as the cap rises 2000→100000, tracking the commit-count climb above.

## Phase 5: repeat verification of Phase 2 close-call cells (with warm-up)

Phase 2's three closest-to-the-3%-bar cells were each single-run measurements. Re-ran each 3x
independently (fresh chain each run) with the new pre-load warm-up step (`warmup_txs`) enabled, to
check the revert calls hold under repeat sampling and aren't single-run noise.

| Cell | Original Δ (single run) | Repeat mean Δ (3x) | Verdict |
|---|---|---|---|
| 0b block-stm-workers 0→4 | +1.37% | **-24%** | Revert — confirmed, and much stronger than the original single run suggested. Original +1.37% was noise; the real effect is a clear regression from CPU contention (5 validators + load generator sharing one host). |
| 2 aggressive consensus timeouts | -2.7% | -2.3% | Revert — confirmed, same direction and magnitude. |
| 3b tx-cache-size 100000→200000 | -4.4% | -3.2% | Revert — confirmed, same direction and magnitude. |

All three Phase-2 revert calls hold. Cell 0b is the standout: single-run measurement badly
understated a real regression — worth treating any single-run "flat/marginal" Phase-2 result with
suspicion going forward.

## Phase 6: libp2p scaler sweep (on top of the R2-fixed `unique-per-tx` config)

`p2p.libp2p.scaler` controls the worker-pool autoscaler for go-libp2p's per-reactor message
dispatch (min/max goroutines, latency threshold to trigger scale up/down, with a separate
MEMPOOL-reactor override — default min 4/max 32 overall, MEMPOOL min 8/max 512). Swept two
directions against the current default (no override) to see whether gossip dispatch is a
bottleneck at 300,000 unique-per-tx senders, single run each (screening):

| Variant | min/max workers | MEMPOOL min/max | threshold_latency | median_tps | failed_txs |
|---|---|---|---|---|---|
| Default (baseline, no override) | 4/32 | 8/512 | 100ms/500ms | 4318.65-4187.77 (mean 4265.34) | 5.2-11.3% |
| A — narrow (less goroutine contention) | 2/8 | 4/64 | default | 4229.02 | 11.3% |
| C — wide/autoscale-headroom (scale up harder under burst) | 8/128 | 16/2048 | 50ms | 4252.23 | 9.8% |

Both variants land inside the baseline's existing run-to-run noise band — no gain, no regression.
**Reverted, no config change kept.** Consistent with R3: on this single-host 5-validator setup,
CPU/execution contention is the wall, not libp2p's message-dispatch concurrency — widening or
narrowing the autoscaler's worker range doesn't touch the actual bottleneck.

## Phase 7: stable 500ms blocktime (infeasible on this hardware)

Hypothesis going in: the ~450ms empty-block floor, not `max_gas`, is what blocks a 500ms
blocktime (fitting all prior load blocks gave `blocktime ~= floor + 0.079ms x txs`, so lowering gas
can't buy a useful 500ms cell if the floor alone eats most of the budget). Tested on the same
5-validator `simple-transfer` setup, v1.8.0-alpha antecache binary, with the R4 client-cap fix
reverted (i.e. the same load-generation config as the Phase 1-6 baseline).

**B1 — measure the floor.** `grep 'txs=0'` on several run logs and took inter-block deltas:
393-764ms, clustering 400-590ms — matching this report's prior citation and confirming the floor
is real and mostly above 400ms at default consensus settings.

**B2 — compress the floor, one knob at a time** (empty-block cadence as the metric, each screened
alone against the same baseline to avoid Phase 2's bundling mistake):

| Knob | Change | Empty-block deltas (steady state) | median_blocktime under load | failed_txs | Verdict |
|---|---|---|---|---|---|
| `skip_timeout_commit` | false -> true | 248-417ms | 1350ms | 14.5% | floor down, load unchanged |
| `timeout_commit` | 200ms -> 50ms | 256-398ms | 1108ms | 12.1% | floor down, load unchanged |
| `peer_gossip_sleep_duration` | 100ms -> 25ms | 338-409ms | 994ms | 19.4% | floor down (smaller), load unchanged |
| all three combined | — | **227-317ms** | 1095ms | 13.7% | floor halved ~450 -> ~250ms; **all reverted**, see recommendation |

The combined figure is steady state (one 598ms outlier at a load-boundary block) — comfortably under
the ~400ms B1 gate, and the largest floor movement found in this investigation.

**But the floor reduction does not reach loaded blocks.** Under full load the combined config gives
median_blocktime 1095ms and median_tps 4296.07, against a baseline of ~1050ms / 4265.34 — both
inside the run-to-run noise band. Halving the *empty*-block cadence bought no measurable
loaded-blocktime or throughput improvement, which is the first sign that the loaded floor is set by
something other than the consensus-round cadence B1 measured (see the B4 verdict below).

**These knobs also cost failure rate.** `failed_txs` runs 12.1-19.4% across the B2 screens against
5.2-11.3% on the immediately preceding R2-fix baseline — consistent with Phase 2's original finding
that faster rounds produce more out-of-order delivery, hence more `sdk:3`.

The `_get_failed_tx_count` denominator was corrected mid-session (see tooling changes), so those two
ranges are not on the same basis as printed — but they are exactly inter-convertible, not merely
comparable. The old denominator was the block's *successful* tx count (from the eth block view, which
omits failures); the new one is all Cosmos txs in the block. Same numerator, so old = `F/S` and new =
`F/(S+F)`, and `simple-transfer` is one EVM tx per Cosmos envelope, making the conversion exact.
Restating the baseline on the new basis gives **4.9-10.2%** against B2's 12.1-19.4% — the increase is
real, roughly 1.5-2x, and if anything the raw figures understate it. No re-measurement needed.

**B3 — bisect `max_gas` under full load**, all three sites (`genesis.consensus.params.block.max_gas`,
`config.mempool.reap_max_gas`, `app-config.mempool.reap_max_gas`) moved together, combined-floor
knobs held constant, 1 run each:

| max_gas | median_tps | gas util | median_blocktime | p95_blocktime | verdict |
|---|---|---|---|---|---|
| 40M | 893.40 | 86.5% | 2523ms | 4142ms | far worse — a 300k-tx backlog draining through an undersized cap takes many more rounds, and the run failed to fully commit (117384/300000) inside the harness's timeout |
| 210M (Phase 1-6 baseline) | 4296.07 | 46.8% | 1095ms | 1605ms | still 2x the 500ms target |
| 320M | 4266.11 | 21.0% | **781ms** | 1493ms | best single point |
| 600M | — | 13.9% | 1128ms | 1511ms | worse than 320M — not monotonic |

The predicted linear model (floor + 0.079ms/tx) does not hold once a 300,000-tx backlog is queued
against the load generator's fixed ~4300 tx/s feed rate: at 210M-600M the block is never actually
gas-bound (gas util 14-47%, capped by send rate, not `max_gas`), so shrinking the cap only forces
more, not faster, rounds to drain the same backlog, and enlarging it past ~320M buys nothing once
blocks already aren't gas-bound. 320M was the best bracket point found.

**B4 — confirm the best point (320M) 3x:**

| Run | median_tps | gas util | median_blocktime | p95_blocktime | notes |
|---|---|---|---|---|---|
| 1 | 4266.11 | 21.0% | 781ms | 1493ms | clean run |
| 2 | 4410.98 | 23.1% | 784ms | 2171ms | clean run |
| 3 | 5425.83 | 92.5% | 2357ms | 3891ms | 3 blocks stalled for 86.7s; only 200338/300000 committed before the harness gave up — `median_tps`/gas-util here are skewed by the stall-excluded window, not comparable to runs 1-2 |

**Verdict: 500ms is not achievable, stably, on this hardware.** The floor-compression knobs (B2) do
reproducibly halve the *empty*-block cadence — confirmed in 3/3 B2 screens, the combined run, and all
three B4 runs, landing empty blocks in the 190-420ms range vs. the ~400-590ms baseline. But that
reduction never reaches a loaded block: under sustained full load the floor is dominated by something
the linear model doesn't capture (likely mempool-depth/CheckTx contention against a 300,000-tx
backlog, not the consensus-round floor B1/B2 measured) — even the best bisection point swings 2/3 of
the time to ~780ms median and 1/3 of the time to a stalling, ~2357ms-median run with a 92.5%-full
mempool. That fails the stability criterion (holds across 3 runs) on its own, well before checking
the 500+-50ms/650ms-p95 bar.

**Recommendation: nothing from Phase 7 is kept.** `scripts/devnet-local/configs/benchmark-5val.jsonnet`
is back to the Phase 1-6 baseline on every knob touched here.

- The B2 consensus knobs (`skip_timeout_commit: true`, `timeout_commit: 50ms`,
  `peer_gossip_sleep_duration: 25ms`) **reverted**. They halve the idle cadence, but that is the only
  thing they do: no loaded-blocktime or throughput movement outside the noise band, and a ~1.5-2x
  increase in `sdk:3` failures. A change that only improves the metric nobody is bound by, while
  degrading one they are, doesn't earn a place in a shared config.
- `max_gas` **reverted to 210M**. 320M's 781ms is the best single point measured but sits inside that
  same config's own 781/784/2357ms spread, and at 21% gas utilization the cap was never binding.

The instability is the finding, not a number to average away. Anyone retrying this should attack the
loaded floor — mempool depth / CheckTx contention against a large backlog — since Phase 7 rules out
the consensus-round cadence as the loaded-blocktime constraint.

## Warm-up (`warmup_txs`) effect on measurement quality

Added a pre-load warm-up step (send+commit `warmup_txs` throwaway txs/account before the measured
window) to prime JIT/connection-pool/mempool state instead of paying that cost inside the measured
run. Tested whether it changes the *TPS number* or just the *measurement's repeatability*, on the
same Phase-2-final config, 3 runs each:

| warmup_txs | median_tps values | mean | range (spread) | failed_txs range |
|---|---|---|---|---|
| 0 | 4648.06, 4239.83, 2352.47 | 3746.8 | 2352-4648 (span 2296) | 12.5-31.7% |
| 10 | 3556.56, 4077.93, 3658.55 | 3764.3 | 3556-4078 (span 522) | 12.9-32.0% |

**Warm-up doesn't move the mean (~0.5% difference, within noise) but visibly tightens the spread**
(span shrinks ~4.4x) — fewer wild cold-start outlier runs. It has no effect on failed_txs (R2's
mempool-rejection issue is unrelated to warm-up). n=3/side is thin, so treat the variance-reduction
claim as directionally consistent with expectation (priming reduces cold-start noise) rather than
statistically proven. Recommendation: keep warm-up on for repeatability, don't read it as a
throughput lever.

## Caveats

- **R1 — app-mempool mempool-status gate is uninformative (fixed).** `/num_unconfirmed_txs` (what
  `--require-saturation` polls) reads 0 the entire run under `mempool.type=app`, regardless of
  actual pending load. Every app-mempool run in this report was gated manually on gas
  utilization + blocktime trend + failed-tx rate instead of `--require-saturation`. `MempoolMonitor`
  now polls the eth `txpool_status` RPC under app-mempool instead (see tooling changes below) —
  future app-mempool runs get a real pending-count signal.
- **R2 — failed_txs sits at 15-29% across every app-mempool cell (root cause fixed, residual ~5-11%
  from a separate mechanism).** Root cause: every failure was `sdk:3` (`ErrInvalidSequence`) at
  DeliverTx. Reusing a fixed sender pool means CometBFT's `BroadcastAsync` (one goroutine per peer
  per envelope, no cross-envelope ordering) can deliver nonce N+1 to a proposer before nonce N;
  `PrepareProposal` runs no ante recheck and trusts each sender's first tx unconditionally, so the
  out-of-order tx goes straight into a block and fails. This is a server-side defect the client
  can't repair by resending correctly — but it *can* be sidestepped: `sender_strategy: unique-per-tx`
  gives every tx its own fresh sender at nonce 0, so there is no nonce sequence left to reorder.
  Applied to the 5-validator config (`5val-simple-transfer.yaml`), plus two supporting fixes: (1)
  dropped the per-sender endpoint-pinning/send-chaining that only matters when a sender repeats
  (`sender_affinity_accounts` → `None` under `unique-per-tx`), which was otherwise forcing
  `broadcast_tx_sync` and serializing 14/15 of all sends for no reason; (2) warm-up is now skipped
  entirely under `unique-per-tx` — genesis funds exactly `num_accounts * num_txs` physical senders,
  the main load already signs every one of them at every offset, and warm-up's own sender-range
  formula (keyed on `warmup_txs`, not `num_txs`) fully overlapped that range, bumping senders to
  nonce 1 that the main load then signed at nonce 0 — a second, self-inflicted source of `sdk:3`
  that produced a full 0/N commit stall until caught.

  Verified on 5 validators, 3 repeats each, `configs/5val-simple-transfer.yaml`:

  | Scale | Run | median_tps | failed_txs | committed/total |
  |---|---|---|---|---|
  | Scaled down (2000 accounts × 15 txs = 30,000 senders, see R3) | 1 | 5595.09 | 0.0% | 30000/30000 |
  | | 2 | 6654.83 | 1.5% | 30459/30000 |
  | | 3 | 6544.76 | 0.0% | 30000/30000 |
  | Full scale (20,000 accounts × 15 txs = 300,000 senders, matches Phase 1-5) | 1 | 4318.65 | 5.2% | 303043/300000 |
  | | 2 | 4289.60 | 5.5% | 303797/300000 |
  | | 3 | 4187.77 | 11.3% | 308825/300000 |

  Full-scale mean **4265.34**, all `sdk:3`, down sharply from 15-29% — but not eliminated, and with
  wide run-to-run variance. This residual is a *different* mechanism: `committed_cosmos_txs`
  exceeded `total_txs` on every run, meaning some sends landed in a block twice. Since
  `unique-per-tx` senders are single-use, a duplicate can only arrive if the send layer resends a
  tx whose first copy already committed — `broadcast_tx_async` gives no completion signal to
  confirm a send actually landed, so under host contention (see R3) a retried duplicate can be
  admitted and committed after the original, and fails DeliverTx with the same nonce it already
  consumed. Not filed as a new root cause here since it's plausibly this host's CPU contention
  inflating retry duplication, not a chain defect — but it means R2's fix is a large improvement,
  not a clean elimination, and the residual number is worth re-checking on quieter hardware.

  **TPS did not improve at the comparable (full 300k-sender) scale**: mean 4265.34 vs. the
  pre-existing Phase 3 `reuse`-mode baseline mean 4623.7 (4494.87/4656.47/4719.78) — about **7.8%
  lower**. Per R4 below, at full scale `median_tps` tracks the load generator's send rate, not the
  chain, so this delta measures the client getting slower with 300,000 distinct sender accounts
  instead of 20,000 (each request costs ~220ms against the larger state tree, vs ~123ms at 30,000
  senders) — not a chain-side throughput regression from the fix.
- **R4 — every full-scale run in this report measures the client's send rate, not the chain.**
  `CONNECTION_POOL_PER_HOST = 200` (`remote_benchmark/transaction.py`) caps aiohttp at 200 in-flight
  requests per RPC endpoint; 5 endpoints x 200 = 1000 in-flight, and at the observed ~220ms/request
  that's ~4545 tx/s — matching every full-scale `median_tps` number above (4187-4862) to within a
  few percent. `peak_mempool_txs` reads 0 on every one of those runs and blocks sit 40-50% full:
  the chain has headroom the client can't fill.

  Made the cap configurable (`send_conn_per_host` in `Config`, threaded into both `send` and
  `send_round_robin`'s `TCPConnector`s — see tooling changes below) and re-ran the R2 full-scale
  config with it raised. **Discriminating result: raising the cap alone made sending slower, not
  faster** (77.7s for 300,000 txs vs the 61.7-70.6s baseline) — the cap was not the true limit; a
  single Python event loop serializing JSON-RPC for 300,000 requests is. Wired the existing
  `send_multiprocess` prototype into `_send_and_report_failures` (`send_workers` config field) to
  fan sending across OS processes instead. That did cut raw send wall-clock 24-45% (46.7s / 38.5s /
  44.7s over 3 runs vs the 61.7-70.6s baseline), confirming the event loop really was part of the
  bottleneck — but it made the *reported* `median_tps` **worse**, not better (2365.12 / 2457.71 /
  2031.48, mean 2285 vs the R2 baseline mean 4265.34): feeding the mempool faster pushed gas
  utilization to 92.5-92.8% and `median_blocktime` to 3271-3426ms (p95 7696-9556ms), far beyond
  what Phase 1's ~450ms-intercept/~0.079ms-per-tx linear fit would predict at that fullness —
  consensus-level contention (vote delays under load) becomes the dominant cost once the client-side
  limit is removed. Per this report's revert-on-no-gain policy (Phase 2, Phase 6), **the
  `send_conn_per_host` / `send_workers` config values were reverted out of
  `configs/5val-simple-transfer.yaml`** — the underlying configurability (`Config` fields,
  `_pool_limit` helper, `send_multiprocess`'s `nonce_ordered` parameter, full test coverage) is kept
  as legitimate infrastructure, just not defaulted on for this workload. `peak_mempool_txs` still
  read 0 on all three multiprocess runs, reconfirming R1's `txpool_status` fix isn't surfacing a
  real pending count under this load shape — recorded, not fixed here.

  **Side finding that closes out R2's residual.** All three multiprocess runs committed exactly
  `300000/300000` Cosmos txs, versus 303043-308825/300000 on every single-process run. R2 could only
  guess that its leftover ~5-11% `sdk:3` came from the send layer resending txs whose first copy had
  already committed; the duplicates vanishing the moment sending stops being the bottleneck confirms
  it. The residual was client-side retry duplication under contention, not a chain defect.
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
- `remote_benchmark/monitor.py` (R1 fix): `MempoolMonitor` now polls the eth `txpool_status` RPC
  under app-mempool instead of the always-zero `/num_unconfirmed_txs`, giving a real pending-tx
  signal. `txpool_status` walks the app-mempool client and is expensive, so it's fetched once per
  new block height rather than every poll tick — each block's peak is a single sample.
- `remote_benchmark/stats.py` (R2 diagnostics): `_get_failed_tx_count` now also returns a
  `Counter({(codespace, code): count})` breakdown of *why* txs failed at execution (ABCI
  code/codespace from `/block_results`), printed per-reason in the load summary.
- `remote_benchmark/runner.py`, `config.py`: added `warmup_txs` (0 = off) — sends and waits for
  `warmup_txs` throwaway txs/account before the measured load window, to prime mempool/JIT/
  connection-pool state instead of paying that cost inside the measured window. Fixed an
  interaction bug where `--txs-cache` reuse skipped warm-up entirely: a cached payload is signed
  assuming warm-up already advanced the nonce, but every rerun starts from a fresh chain (nonce
  back at 0), so a cache hit compared against a stale pre-warm-up nonce and failed validation.
  Warm-up now always runs before the cache-hit check. (R2 fix) `_run_warmup` now skips entirely
  under `sender_strategy: unique-per-tx` instead of advancing the nonce past `warmup_txs` — that
  advance assumed reused senders; unique-per-tx senders are single-use and fresh at nonce 0, and
  warm-up's sender range fully overlapped the main load's, so the old behavior signed the main
  load's txs at the wrong nonce for every sender and stalled the run at 0 commits.
- `remote_benchmark/config.py`, `transaction.py` (R2 fix): added `sender_affinity_accounts`,
  returning `None` for `unique-per-tx` and `num_accounts` otherwise; wired into `send_round_robin`
  call sites in `runner.py`, `cli.py`, and `soak.py` so a sender that never repeats isn't pinned to
  one endpoint or chained through `broadcast_tx_sync` for no reason.
- `scripts/devnet-local/run-benchmark.sh`: added `MEMPOOL_SIZE` env override for the legacy-
  mempool jsonnet configs (sed-patches `mempool.size`, cleaned up on exit) — used for the Phase 4
  sweep.
- `remote_benchmark/config.py` (R4): added `send_conn_per_host: int = 200` and
  `send_workers: int = 1`, both defaulting to prior behavior.
- `remote_benchmark/transaction.py` (R4): `send` and `send_round_robin` take a `conn_per_host`
  kwarg (default `CONNECTION_POOL_PER_HOST`) instead of hardcoding the connector cap. Added
  `_pool_limit(conn_per_host, n_hosts)` so the connector's total `limit` can't bind below the
  per-host aggregate. `send_multiprocess` (previously an unwired prototype) now takes a
  `nonce_ordered` flag so it can fan out `unique-per-tx` sends (no per-sender nonce ordering to
  protect, `num_accounts=None` passed through) as well as `reuse`-mode sends (forces `sync=True`
  and a per-worker `batch_size` to protect nonce ordering across workers).
- `remote_benchmark/runner.py`, `cli.py` (R4): `_send_and_report_failures` dispatches to
  `send_multiprocess` when `send_workers > 1`, else routes through `send_round_robin` unchanged.
  `conn_per_host` threaded into every `send_round_robin`/`send_multiprocess` call site (main load,
  warm-up, `send-txs`, `soak`) via `getattr(cfg, "send_conn_per_host", 200)` for compatibility with
  existing fake configs in `tests/test_cli.py`.
- Tests added: `tests/test_config.py` (defaults for both new fields), `tests/test_transaction.py`
  (`conn_per_host` reaches the connector, the total-limit floor, `send_multiprocess`'s
  `nonce_ordered` branches), `tests/test_runner.py` (dispatch to `send_round_robin` vs.
  `send_multiprocess`). Full suite: 257 passing (was 248 before this investigation).

## Raw data

Per-run logs, JSON result records, and devnet data-dir paths for every run cited above are in
`/tmp/bench/NOTES.md` and `/tmp/bench/*.json` / `*.log`.
