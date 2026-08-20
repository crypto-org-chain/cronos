# Plan: diagnose and fix the benchmark "stall" pattern

## Part 0 — What "stall" actually means in this tooling

"Stall" is **not** a mempool-empty / no-load-committed condition. It is a purely relative block-time outlier test:

- `remote-benchmark/remote_benchmark/window.py:157-167` — `q1 = quantiles(block_times, n=4)[0]`, `stall_threshold = q1 * stall_mult` (`stall_mult=5`, default at `window.py:100`). Any interval with `bt > stall_threshold` is a "stall". Requires `len(block_times) >= 4`, else no stall detection at all.
- `window.py:190-195` — stalled intervals' time **and** their block's txs/gas are subtracted, giving `adjusted_duration` / `steady_txs`; `overall_tps` is computed on the steady remainder only.
- `window.py:245` — `stall_height_offsets`; printed by `remote-benchmark/remote_benchmark/stats.py:127-135` (`stalls_excluded`) and `stats.py:181-190` (`load_period ... (steady Xs, stall Ys)`).

Consequences that matter for the investigation:

1. It is scale-free, so it conflates two unrelated things. Compare `remote-benchmark/local/report/v180a-unique-per-tx-remote-run1.log` (`stalls_excluded 3 blocks (13.4s)`, median blocktime 792ms, `round_increments 0`, run completed) — a benign warm-up-ramp artifact — with `remote-benchmark/local/report/b4-confirm-320M-run3.log:142` (`stalls_excluded 3 blocks (86.7s) at heights [3149, 3150, 3151]`), which is a **chain halt**, not a slow block.
2. Stalled blocks are excluded from `median_tps`, `median_blocktime`, `p95/p99`, and `overall_tps`, so a stalling run reports *better* per-block numbers than a healthy one. `b4-confirm-320M-run3.log` reports `median_tps 5425.83` — the best of the three B4 runs — while committing the least (`200338/300000`).

### What actually happened in the canonical stall run

From `b4-confirm-320M-run3.log:118-134`, block times in order: 193, 324, 1071, 898, 1549, 2258, 2345, 2369, 3167, 3339, 3643, 3331, 3510, 4350, **29444, 15994, 41297** ms.

- The three "stall" blocks are **full of successful txs**: `block 3151 txs=14089 gas=295869000` = exactly 14089 × 21000. Blocks 3149/3150 likewise ~15k txs at ~318M gas against the 320M cap. So this is not starvation, not an empty mempool, and not mostly-failing blocks — it is ~15k real txs taking 29-41s to commit where the immediately preceding identical-size blocks took 3.3-4.4s.
- After 3151 the chain **stops entirely**: `b4-confirm-320M-run3.log:109` — `chain only advanced to 3151 from 3151 within 60s ... {'node0': 3151, ..., 'node4': 3151}`, then `:110 Error: timed out waiting for generated transactions to commit: 200338/300000`.
- Client load was already finished (`:70` `sent 300000/300000 txs, 56.6s elapsed`) and the commit-waiter was idle-polling (`:71-108`). **The stall is chain-side, not client pacing.**
- `mempool=0` on every block line: the run was blind to mempool depth (see Part 2, gap G1).

So the phenomenon to fix is: **a congestion cliff where full-size blocks go from ~3.5s to ~30-40s and then the chain wedges**, and it is *bimodal* — 2 of 3 B4 runs never hit it.

## Part 1 — What the historical sweep data says

`matrix-results.csv` is useless here (4 rows, all `status=failed`, columns `binary,validators,testcase,round,overall_tps,status,log`). The mempool-size sweep and the b2/b3/b4 logs are the real data.

**A. Per-block cost scales monotonically with mempool depth** (`mempool-size-sweep-size-*.log`, identical 300k-tx workload, legacy mempool):

| `mempool.size` | median_blocktime | overall_tps | peak_mempool_txs | source |
|---|---|---|---|---|
| 2000 | 629ms | 1191.11 | 2038 | `mempool-size-sweep-size-2000.log:397091,397072,397103` |
| 10000 | 1014ms | 2048.50 | 10007 | `...-10000.log:104736,104717,104748` |
| 50000 | 3135ms | 1663.91 | 50001 | `...-50000.log:49934,49915,49946` |
| 100000 | 3358ms | 1244.33 | 100000 | `...-100000.log:67659,67640,67671` |

Blocktime rises 5.3x as the cap rises 50x, and TPS peaks at 10000 then *falls*. Deeper pool = strictly more work per block. This is the mechanism that makes the cliff self-reinforcing once a backlog forms.

**B. Stalling correlates with block occupancy, not with any consensus knob.** Cross-run table (all 5val, v1.8.0-alpha, app-mempool, 300k txs):

| run | max_gas | gas util | median_bt | failed% | committed | stall |
|---|---|---|---|---|---|---|
| `b2-combined-floor.log` (skip_timeout_commit+50ms commit+25ms gossip) | 210M | 46.8% | 1095ms | 13.7% | 302784 | none |
| `b3-gas-320M.log` | 320M | 21.0% | 781ms | 21.3% | 305938 | none |
| `b3-gas-600M.log` | 600M | 13.9% | 1128ms | 19.8% | 302867 | none |
| `b3-gas-40M.log` | 40M | 86.5% | 2523ms | — | 117384 | run failed to commit |
| `b4-confirm-320M-run2.log` | 320M | 23.1% | 784ms | 14.0% | 305400 | none |
| `b4-confirm-320M-run3.log` | 320M | **92.5%** | 2357ms | 3.3% | **200338** | **86.7s + halt** |

Two things fall out of this that no prior write-up states:

- **The consensus-knob sweeps (b2: `timeout_commit` 50ms, `skip_timeout_commit`, `peer_gossip_sleep_duration` 25ms) do not correlate with stalls at all** — none of them stalled, and none of them helped. Consistent with the tuning-doc's own Phase-7 verdict (`git show 5c090c47^:tuning-doc/benchmark-report-devnet-tps.md`, Phase 7 B2/B4): "Phase 7 rules out the consensus-round cadence as the loaded-blocktime constraint."
- **Every non-stalling run committed *more* Cosmos txs than were sent** (302k-306k vs 300000) with 14-21% ABCI failures, while the stalling run had the *lowest* failure rate (3.3%) and the *highest* occupancy. So the "healthy" runs are quietly burning 14-21% of block space on duplicate/nonce-failing txs, which keeps real occupancy at 13-47% and keeps them below the cliff. Run 3 is the one that actually packed blocks with real load — and died. **The two phenomena (stall vs. chain-slower-than-sender) are linked by this: the sender-bound runs never test the chain at full occupancy.**

**C. Round escalation under load on this host is real and documented in node logs.** `local/report/lp2p-fix-5val-batch-nodelogs/node0.log` — `round=1` ×45, `round=2` ×12, `round=3` ×3, `round=4` ×1. Height 63 in full (`node0.log:994-1092`): proposal received, then `Timed out dur=1000 step=RoundStepPrevoteWait`, `Timed out dur=1000 step=RoundStepPrecommitWait`, → `round=1` with new proposer, `Timed out dur=3500 step=RoundStepPropose`, → `round=2`, then commit — for a **172-tx** block. Also note `Timed out dur=-81436.941 height=63 step=RoundStepNewHeight` and `dur=-15107/-20383/-24822` at heights 64-66: the node entered each new height tens of seconds *behind* the previous block's timestamp, i.e. the commit pipeline was lagging wall-clock badly. And `node0.log:1393-1395`: `creating state snapshot height=65` followed by `ERR failed to create state snapshot err="...snapshot is not created yet: height: 65"` (memiavl `snapshot-interval: 5`, `benchmark-5val.jsonnet:124`).

## Part 2 — Instrumentation gaps that made this un-diagnosable

**G1 — mempool depth is always 0 in local runs.** `runner.py:442-445` enables the app-mempool-aware sampler only when the run YAML declares `node_config: {mempool.type: app}`:
```python
is_app_mempool = (getattr(cfg.primary, "node_config", None) or {}).get("mempool.type") == "app"
mempool_monitor = MempoolMonitor(..., json_rpc=cfg.primary.json_rpc_candidates[0] if is_app_mempool else None, ...)
```
`scripts/devnet-local/configs/5val-simple-transfer.yaml` declares no `node_config`, while `benchmark-5val.jsonnet:25-31` sets `mempool.type: 'app'`. So the monitor falls back to `mempool_status()` → `/num_unconfirmed_txs`, which is documented to always read 0 under `type=app` (`utils.py:191-199`). Every `mempool=...` figure in b2/b3/b4 is a false zero. **The single most important variable in hypothesis A/B was never measured.**

**G2 — no consensus/round/ABCI-stage metrics in local runs.** `5val-simple-transfer.yaml` has no `telemetry:` key, so `dump_block_stats` skips the Prometheus sections entirely (`stats.py:684-695`). Those sections exist and work — see `local/report/v180a-scaled-accounts-remote-run1.log` (`avg_abci_finalize_block`, `avg_abci_commit`, `avg_block_processing`, `avg_step_propose/prevote/precommit/commit`, `avg_block_interval`, `round_increments`, `current_round`, `late_votes`, `block_gossip_parts_mismatched`) — but only 3 of ~279 report files have them, all remote.

**G3 — no cronos app-mempool telemetry is scraped at all.** `grep -rn "cronos_" remote_benchmark/*.py` returns nothing. The chain already emits the discriminating counters: `cronos_mempool_pool_size` and `cronos_mempool_pool_snapshot` timing (`app/mempool/manager.go:353`, `app/mempool/helpers.go:14`), `cronos_mempool_reap_gossip_sent` / `..._deduped` / `..._encode_cache_hit|miss` (`app/mempool/reap.go:68-76`), `cronos_mempool_prepare_encode_cache_hit|miss` (`app/proposal.go:186`), `cronos_mempool_recheck_*` (`manager.go:410-413,506`). None reach the report.

**G4 — per-block failure counts are computed and thrown away.** `stats.py:275-286` builds `failed_counts = {height: (failed, included)}` for every height, then only aggregates into `total_failed_txs`. `_print_blocks` (`stats.py:332-340`) prints txs/gas/mempool but not failures.

**G5 — the 100%-failed-block detector postdates all these runs.** `eth_indexer_gap_txs` (`stats.py:270-274, 667-671`) landed in `c3e2018b`, dated 2026-08-19 17:11; the b2/b3/b4 runs are from 2026-08-16.

## Part 3 — Ranked root-cause hypotheses, each with a falsifying prediction

**H1 (primary) — commit/storage-pipeline backpressure cliff.** Once ~200k txs of writes have accumulated, memiavl async-commit + rocksdb hit a write-stall/compaction cliff; `Commit`/`FinalizeBlock` latency jumps ~10x, block times go 3.5s → 30-40s, the commit pipeline (`async-commit-buffer: 16`, `benchmark-5val.jsonnet:126`; `snapshot-interval: 5`, `:124`) backs up and the chain wedges. Direct evidence: negative `RoundStepNewHeight dur=-81436/-15107/-20383/-24822` values and ~1-minute lag between `finalizing commit height=63` and `committed state height=63` (`lp2p-fix-5val-batch-nodelogs/node0.log:1086-1182`), plus `failed to create state snapshot` error at `:1395`. **Prediction: `avg_abci_commit` and/or `avg_abci_finalize_block` dominate the stall window; `round_increments` stays ~0.**

**H2 — consensus round escalation on full-size blocks.** A ~15k-tx / ~4.5MB block can't gather prevote/precommit quorum inside the 1s windows when 5 validators plus the load generator share one host, so heights burn 6-10 rounds at escalating deltas. Supported by `node0.log:1017-1092` (rounds 1-2 on a *172-tx* block) and `benchmark-5val.jsonnet:6-8`'s own note that a 363M block "does not propagate across five validators inside the round timeouts". **Prediction: `round_increments` > 0 and `current_round` > 0 during the stall; time sits in `step_propose`/`step_prevote`/`step_precommit`.**

**H3 — O(pool) gossip-reap amplification.** `NewReapTxsHandler` (`app/mempool/reap.go:24-70`) runs `PoolSnapshot` — a full priority-index walk allocating a slice of the whole pool (`helpers.go:14-21`) — on **every** `reap_interval` tick (500ms, `benchmark-5val.jsonnet:30`). The gossip-TTL dedupe path `continue`s *without* counting (`reap.go:63-66`), so `maxPerReap` (`:117`) never breaks early once most txs are already gossiped — the loop traverses the entire pool every tick, doing `EncodeTx`+`HashTx` per tx. With `tx-cache-size: 100000` (`:118`) against a >100k pool, `EncoderCache` thrashes, contending on `EncoderCache.mu` (`encoder_cache.go:48-131`) that admission and PrepareProposal also need. `tracker.prune` additionally iterates the whole `seen` map under its mutex every reap (`gossip.go:46-54`). **Prediction: `cronos_mempool_pool_snapshot` timing and `..._reap_encode_cache_miss` climb steeply with pool depth; `..._reap_gossip_deduped` >> `..._sent` during the stall.**

**H4 — zombie-tx retention (a stall amplifier, and separately explains the remote "100% invalid nonce" flood).** `mempool.recheck: false` (`benchmark-5val.jsonnet:24`) disables all post-commit eviction including TTL (`app/mempool/manager.go:337-340`, warning at `app/app.go:573`). The fast PrepareProposal path does no ante re-check (`app/proposal.go:179-190`, wired `app/app.go:545`). In `baseapp.go`, ante-failure returns at `:912`, *before* `mempool.RemoveWithReason(...)` at `:935-939` — a tx that fails ante during FinalizeBlock is never removed. Loop: stale tx → proposed unchecked → fails `sdk:3` in FinalizeBlock → not removed → re-proposed forever. Matches the 14-21% `sdk:3` in every non-stalling run (with `unique-per-tx` sender strategy, `sdk:3` can only mean re-inclusion of an already-committed tx) and `committed_cosmos_txs 305938/300000` in `b3-gas-320M.log` (proven duplicate inclusion). **Prediction: per-block failure counts show a rising floor concentrated in specific blocks; `cronos_mempool_pool_size` fails to drain toward zero.**

**H5 (cheap long-shot) — event-bus backpressure.** `event_bus_buffer_capacity: 128` (`benchmark-5val.jsonnet:19`) against 15000 tx events per block. Cheap to test.

Explicitly **de-prioritized**: consensus timeout tuning, libp2p scaler, block-stm worker count — all already swept with null results (tuning-doc Phases 2/5/6; `lp2p-scaler-varA-run1.log`, `lp2p-scaler-varC-run1.log`; `b2-*.log`). Do not re-sweep.

## Part 4 — Implementation plan

### Phase A — instrumentation (before touching any chain config)

- **A1.** `scripts/devnet-local/configs/5val-simple-transfer.yaml`: add `node_config: {mempool.type: app}` (engages `txpool_status` sampling) and `telemetry: <prometheus url>` (emits Consensus Stage Timing / Consensus Health / Mempool Health sections). Verify first that a local 5-node devnet exposes per-node Prometheus — `benchmark-5val.jsonnet:56-59` sets one fixed `:9090` addr for all five nodes; may need per-node templating, else accept node0-only scraping.
- **A2.** `remote_benchmark/stats.py:332-340` (`_print_blocks`): print per-block `failed=` from `failed_counts` (`stats.py:275-286`), mark stall blocks inline. Closes G4.
- **A3.** New scraper in `remote_benchmark/cometbft_metrics.py` for `cronos_mempool_pool_size`, `cronos_mempool_pool_snapshot`, `cronos_mempool_reap_gossip_sent|deduped`, `cronos_mempool_reap_encode_cache_hit|miss`, `cronos_mempool_prepare_encode_cache_hit|miss`, `cronos_mempool_recheck_evicted|expired|ttl_expired|proposal_timeout`, from the app telemetry endpoint (`app-config.telemetry`, `benchmark-5val.jsonnet:104-107`) — exact names must be read off a live `/metrics?format=prometheus`. Closes G3.
- **A4.** `remote_benchmark/window.py:157-167`: add an absolute guard and a `stall_kind` classification (slow-but-committing / chain-wedge / warm-up-ramp) alongside the existing 5×Q1 rule. Touches `stats.py:127-135`, `results.py:129`, `compare.py:20`, `tests/test_stats_window.py:36-47`, `tests/test_results.py:183-206`.
- **A5.** Add optional node-log capture to `scripts/devnet-local/run-benchmark.sh` (`--keep-node-logs`) — round escalation is only visible there.

### Phase B — reproduce and classify (no fixes yet)

- **B1.** Re-run the b4 config (5val, max_gas/reap_max_gas=320M, baseline otherwise) on HEAD with Phase A landed, N=6 runs, keeping node logs.
- **B2.** On a captured stall, read off: ABCI-stage dominance → H1; round_increments>0 → H2; block_interval >> Σ(stages) with round_increments~0 → gossip/quorum wait; pool_snapshot timing + encode_cache_miss climbing with depth → H3; failed floor + non-draining pool → H4. Record `txpool_status` trajectory in all cases.
- **B3.** Independently confirm H4 regardless of B2: diff committed tx hashes against sent tx hashes to quantify re-inclusion.

### Phase C — fixes, in dependency order

- **C0 (unconditional, cheapest, alongside B1).** Flip `mempool.recheck: true` in `benchmark-5val.jsonnet:24` as an A/B arm. Metric: failed% floor should drop from 14-21% toward ~0; `committed_cosmos_txs` should stop exceeding sent count.
  - **Result: confirmed, landed as the new default.** 6-run A/B (`c0-recheck-true-run{1..6}.log`): 4/6 zero-failure/exact-commit, 1/6 hit the still-open H1 wedge (unrelated), 1/6 had a 0.1% floor. ~2 orders of magnitude better than baseline's 14-21% floor.
- **C1 if H1.** Config screens: `memiavl.snapshot-interval` 5→100+; `async-commit-buffer` 16→larger or 0 (diagnostic); confirm rocksdb `node_type: validator` profile applied. Only profile `Commit` directly if config screens don't move it.
  - **Result: confirmed, landed as the new default.** `snapshot-interval` 5→100 did not move it (`c1-snapshot100-run{1,2}.log`: both still stalled, run2 wedged so hard it only committed 193380/300000). `async-commit-buffer` 16→0 (forces synchronous commit) cleared the wedge in 3/3 runs (`c1-asyncbuf0-run{1,2}.log`, `c-combined-run3.log`): 0 stalls, 0 failed txs, exact 300000/300000 commit, `round_increments 0`, median TPS 4316-4363 (matches the 4265-4411 baseline band, no regression). Root cause is the async commit queue itself, not snapshot frequency — no need to profile `Commit` directly.
- **C2 if H2.** Do not re-tune consensus timeouts (already null-swept). Add a hard tx-count ceiling on proposals — there is currently no cap on `PrepareProposal` besides `MaxTxBytes`/`MaxGas`; add `max-txs-per-proposal` in `app/proposal.go:130-165` (`NewMempoolProposalHandler`/`ExtTxSelector`).
- **C3 if H3.** In `app/mempool/reap.go`: (a) count TTL-deduped txs against the scan budget so the loop can't traverse the whole pool (the `continue` at `:65` bypasses the `maxPerReap` break at `:69`); (b) avoid a full `PoolSnapshot` per tick — bound the scan or track "not yet gossiped" separately. Plus config: raise `tx-cache-size` to ≥ offered load — but tuning-doc Phase 5 measured 100000→200000 at **-3.2%**, so screen, don't assume.
- **C4 if H4 confirmed (correctness bug, worth fixing regardless of stall outcome).** Evict on FinalizeBlock ante failure from cronos's own hook (cleaner than re-checking ante on the cached proposal path, which reintroduces CPU cost that path exists to avoid) — `baseapp.go:912` returns before the `execModeFinalize` removal at `:935`.
- **C5 if H5.** `event_bus_buffer_capacity` 128 → 32768 (`benchmark-5val.jsonnet:19`), single-knob screen.

## Part 5 — Acceptance criteria

Negative control: `b4-confirm-320M-run3.log`. Positive controls: `b4-confirm-320M-run2.log`, `b3-gas-320M.log`.

1. **No wedge.** 6/6 runs reach `committed_cosmos_txs >= 300000`, no wedge/timeout warnings.
2. **No superlinear block.** `slowest_blocktime` stays within ~3x `median_blocktime` in 6/6 runs, or `stall_kind` is only `ramp-artifact` (needs A4).
3. **Occupancy is no longer the trigger.** At least 2 runs reach `median_gas_utilization >= 80%` without stalling (today's runs "passed" only at 21-23%).
4. **`failed_txs` drops below ~1%**, `committed_cosmos_txs` never exceeds sent count.
5. **Mempool depth observable and drains**: `peak_mempool_txs > 0`, `end_mempool_txs` → ~0.
6. **Regression guard**: `median_tps` no worse than the 4265-4411 band from `b3-gas-320M.log`/`b4-confirm-320M-run2.log` — do not target run 3's stall-inflated 5425.83.

## Part 6 — Risks

- **Bimodality means N=1 proves nothing** (stall reproduces ~1/3 of the time) — every Phase C screen needs ≥3 runs; acceptance bar is 6 runs.
- **A4 changes a metric that `compare.py`/`results.py` persist** — historical JSON records become non-comparable on `stall_*` fields; keep the raw 5×Q1 set alongside the new classification.
- **Local 5val Prometheus port may be unbindable across 5 nodes on one host** — if so, A2/B2 degrade to node0-only.
- **Single-host contention is a confounder for H2** — if B2 points at H2, re-confirm on the remote 5-node devnet before committing to the C2 code change; remote runs with metrics already show `round_increments 0`, arguing against H2 there.

### Critical files
- `remote-benchmark/remote_benchmark/window.py`
- `remote-benchmark/remote_benchmark/stats.py`
- `remote-benchmark/scripts/devnet-local/configs/5val-simple-transfer.yaml`
- `remote-benchmark/scripts/devnet-local/configs/benchmark-5val.jsonnet`
- `app/mempool/reap.go`
- `app/proposal.go`
