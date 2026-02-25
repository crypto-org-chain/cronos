import logging
from datetime import datetime, timezone
from statistics import median, quantiles

from .utils import (
    LOCAL_JSON_RPC,
    LOCAL_RPC,
    block,
    block_eth,
    block_height,
    block_results,
    mempool_status,
)

log = logging.getLogger(__name__)

# sliding window size for per-block TPS calculation
TPS_WINDOW = 10

LOCAL_TELEMETRY = "http://127.0.0.1:26660"


def calculate_tps(blocks, anchor_is_separate=True):
    """
    Calculate TPS for a sequence of blocks.

    blocks: list of (tx_count, timestamp) tuples, ordered by height.

    When anchor_is_separate is True (default), blocks[0] is a pure time
    anchor whose txs belong to a prior period; only blocks[1:] txs are
    counted over the interval blocks[0].timestamp .. blocks[-1].timestamp.

    When anchor_is_separate is False, blocks[0] is itself a transaction
    block with no preceding anchor available; all blocks' txs are counted
    over the same time interval.
    """
    if len(blocks) < 2:
        return 0

    counted = blocks[1:] if anchor_is_separate else blocks
    txs = sum(n for n, _ in counted)
    _, t1 = blocks[0]
    _, t2 = blocks[-1]
    time_diff = (t2 - t1).total_seconds()
    if time_diff == 0:
        return 0
    return txs / time_diff


def get_block_info_cosmos(height, rpc):
    blk = block(height, rpc=rpc)
    timestamp = datetime.fromisoformat(blk["result"]["block"]["header"]["time"])
    txs = len(blk["result"]["block"]["data"]["txs"])
    return timestamp, txs


def get_block_info_eth(height, json_rpc):
    blk = block_eth(height, json_rpc=json_rpc)
    timestamp = datetime.fromtimestamp(int(blk["timestamp"], 0), tz=timezone.utc)
    txs = len(blk["transactions"])
    return timestamp, txs


def _extract_gas(eth_blk):
    """Extract (gas_used, gas_limit) from an already-fetched eth block dict."""
    gas_used = int(eth_blk.get("gasUsed", "0x0"), 16)
    gas_limit = int(eth_blk.get("gasLimit", "0x0"), 16)
    return gas_used, gas_limit


def _get_failed_tx_count(height, rpc):
    """Return the number of failed txs from CometBFT block_results."""
    try:
        res = block_results(height, rpc=rpc)
        tx_results = res.get("result", {}).get("txs_results") or []
        return sum(1 for r in tx_results if int(r.get("code", 0)) != 0)
    except Exception:
        log.debug("block_results unavailable for height %d", height, exc_info=True)
        return 0


def get_block_info_hybrid(height, rpc, json_rpc):
    """
    Use Cosmos RPC for timestamps (sub-second precision) and
    Ethereum JSON-RPC for tx counts and gas data.

    Returns (timestamp, tx_count, gas_used, gas_limit).
    """
    cosmos_blk = block(height, rpc=rpc)
    timestamp = datetime.fromisoformat(cosmos_blk["result"]["block"]["header"]["time"])
    eth_blk = block_eth(height, json_rpc=json_rpc)
    txs = len(eth_blk["transactions"])
    gas_used, gas_limit = _extract_gas(eth_blk)
    return timestamp, txs, gas_used, gas_limit


def _fetch_prometheus(telemetry_url=LOCAL_TELEMETRY):
    """Fetch raw Prometheus text from the /metrics endpoint.

    Returns the response text, or empty string if unavailable.
    """
    import requests as _requests

    try:
        resp = _requests.get(f"{telemetry_url}/metrics", timeout=5)
        resp.raise_for_status()
        return resp.text
    except Exception:
        return ""


def _parse_histogram_avg(lines, metric_name, label_filter=None):
    """Compute average from a Prometheus histogram's _sum and _count lines.

    Returns (avg, count) or (None, 0) if not found.
    """
    total = None
    count = 0
    for line in lines:
        if line.startswith("#"):
            continue
        if label_filter and label_filter not in line:
            continue
        if f"{metric_name}_sum" in line:
            total = float(line.split()[-1])
        elif f"{metric_name}_count" in line:
            count = int(float(line.split()[-1]))
    if total is not None and count > 0:
        return total / count, count
    return None, 0


def scrape_blockstm_metrics(prom_text):
    """Parse block-stm gauges from Prometheus text."""
    result = {}
    for line in prom_text.splitlines():
        if line.startswith("#"):
            continue
        if "blockstm_executed_txs" in line:
            result["executed_txs"] = float(line.split()[-1])
        elif "blockstm_validated_txs" in line:
            result["validated_txs"] = float(line.split()[-1])
    return result


def scrape_consensus_metrics(prom_text):
    """Parse CometBFT consensus stage timings from Prometheus text.

    Returns dict mapping stage names to (avg_seconds, sample_count).
    """
    lines = prom_text.splitlines()
    result = {}

    # consensus step durations (Propose, Prevote, Precommit, Commit, etc.)
    for step in (
        "NewHeight",
        "NewRound",
        "Propose",
        "Prevote",
        "PrevoteWait",
        "Precommit",
        "PrecommitWait",
        "Commit",
    ):
        avg, cnt = _parse_histogram_avg(
            lines,
            "cometbft_consensus_step_duration_seconds",
            label_filter=f'step="{step}"',
        )
        if avg is not None:
            result[f"step_{step}"] = (avg, cnt)

    # FinalizeBlock processing time (histogram in ms)
    avg, cnt = _parse_histogram_avg(lines, "cometbft_state_block_processing_time")
    if avg is not None:
        result["finalize_block_ms"] = (avg, cnt)

    # ABCI method timings (seconds) – FinalizeBlock and Commit
    for method in ("finalize_block", "commit"):
        avg, cnt = _parse_histogram_avg(
            lines,
            "cometbft_abci_connection_method_timing_seconds",
            label_filter=f'method="{method}"',
        )
        if avg is not None:
            result[f"abci_{method}"] = (avg, cnt)

    # block interval
    avg, cnt = _parse_histogram_avg(lines, "cometbft_consensus_block_interval_seconds")
    if avg is not None:
        result["block_interval"] = (avg, cnt)

    # quorum delays
    for line in lines:
        if line.startswith("#"):
            continue
        if "cometbft_consensus_quorum_prevote_delay" in line:
            result["quorum_prevote_delay"] = (float(line.split()[-1]), 1)
        elif "cometbft_consensus_quorum_precommit_delay" in line:
            result["quorum_precommit_delay"] = (float(line.split()[-1]), 1)

    return result


def dump_block_stats(
    fp,
    eth=True,
    json_rpc=LOCAL_JSON_RPC,
    rpc=LOCAL_RPC,
    start: int = 2,
    end: int = None,
    mempool_data: dict = None,
    stm_data: dict = None,
):
    """
    Dump per-block stats and summary metrics.

    Reports per-block data and a summary section with:
    - TPS: peak, overall, median
    - Gas throughput: GPS (gas per second), peak GPS
    - Gas utilization: median gas_used / gas_limit ratio
    - Per-tx gas: avg, median, max
    - Block time: median, fastest, slowest
    - Failed tx count/ratio
    - Block-STM re-execution ratio (if telemetry is available)

    mempool_data: optional dict {block_height: (peak_n_txs, peak_n_bytes)}
        collected during the load period by a MempoolMonitor. When provided,
        gives accurate per-block mempool snapshots instead of a post-hoc
        query that always sees an empty mempool.

    stm_data: optional dict {block_height: (executed_txs, validated_txs)}
        collected during the load period by a BlockSTMMonitor. Block-STM
        uses Prometheus gauges (overwritten each block), so post-hoc scraping
        only sees the last block's value. This dict provides per-block data.
    """
    if end is None:
        end = block_height(rpc)

    blocks = []
    gas_data = []
    per_tx_gas_values = []
    total_failed_txs = 0
    total_counted_txs = 0
    mempool_snapshots = []

    prev_timestamp = None
    for i in range(start, end + 1):
        if eth:
            timestamp, txs, gas_used, gas_limit = get_block_info_hybrid(
                i, rpc, json_rpc
            )
        else:
            timestamp, txs = get_block_info_cosmos(i, rpc)
            gas_used, gas_limit = 0, 0

        if txs > 0:
            total_failed_txs += _get_failed_tx_count(i, rpc)
            total_counted_txs += txs
            per_tx_gas_values.append(gas_used // txs)
        gas_data.append((gas_used, gas_limit))
        blocks.append((txs, timestamp))

        if mempool_data is not None:
            mp_txs, mp_bytes = mempool_data.get(i, (-1, -1))
        else:
            try:
                mp_txs, mp_bytes = mempool_status(rpc)
            except Exception:
                mp_txs, mp_bytes = -1, -1
        mempool_snapshots.append((mp_txs, mp_bytes))

        tps = calculate_tps(blocks[-TPS_WINDOW:])
        mp_str = f" mempool={mp_txs}" if mp_txs >= 0 else ""
        if prev_timestamp is not None:
            bt = (timestamp - prev_timestamp).total_seconds()
            bt_ms = bt * 1000
            gas_str = f" gas={gas_used}" if gas_used > 0 else ""
            print(
                f"block {i} txs={txs}{gas_str}"
                f" {timestamp} {bt_ms:.0f}ms tps={tps:.2f}{mp_str}",
                file=fp,
            )
        else:
            print(
                f"block {i} txs={txs} {timestamp} - tps={tps:.2f}{mp_str}",
                file=fp,
            )
        prev_timestamp = timestamp

    # --- Summary statistics ---
    first_tx_idx = None
    last_tx_idx = None
    for idx, (txs, _) in enumerate(blocks):
        if txs > 0:
            if first_tx_idx is None:
                first_tx_idx = idx
            last_tx_idx = idx

    print(file=fp)

    if first_tx_idx is None:
        print("no_load_period", file=fp)
        return

    multi_block = last_tx_idx is not None and first_tx_idx < last_tx_idx
    anchor_is_separate = first_tx_idx > 0
    anchor_idx = first_tx_idx - 1 if anchor_is_separate else first_tx_idx
    load_blocks = blocks[anchor_idx : last_tx_idx + 1]
    load_gas = gas_data[anchor_idx : last_tx_idx + 1]

    load_tps_values = []
    load_gps_values = []
    block_times = []
    for j in range(1, len(load_blocks)):
        _, t_prev = load_blocks[j - 1]
        _, t_curr = load_blocks[j]
        bt = (t_curr - t_prev).total_seconds()
        block_times.append(bt)

        if bt > 0:
            gu, _ = load_gas[j]
            load_gps_values.append(gu / bt)

        win_start = max(0, j + 1 - TPS_WINDOW)
        window = load_blocks[win_start : j + 1]
        if len(window) >= 2:
            win_has_anchor = anchor_is_separate or win_start > 0
            load_tps_values.append(
                calculate_tps(window, anchor_is_separate=win_has_anchor)
            )

    # --- Detect stalled blocks ---
    # Use the 25th-percentile block time as the "normal" baseline.
    # Blocks slower than STALL_MULT × baseline are stalls (e.g. tx-flood
    # overwhelming the proposer) and are excluded from timing summaries.
    stall_mult = 5
    stall_indices = set()
    if len(block_times) >= 4:
        q1 = quantiles(block_times, n=4)[0]
        stall_threshold = q1 * stall_mult
        for j, bt in enumerate(block_times):
            if bt > stall_threshold:
                stall_indices.add(j)

    steady_block_times = [
        bt for j, bt in enumerate(block_times) if j not in stall_indices
    ]
    steady_tps_values = [
        v for j, v in enumerate(load_tps_values) if j not in stall_indices
    ]
    steady_gps_values = [
        v for j, v in enumerate(load_gps_values) if j not in stall_indices
    ]

    counted = load_blocks[1:] if anchor_is_separate else load_blocks
    total_txs = sum(n for n, _ in counted)
    _, t_start = load_blocks[0]
    _, t_end = load_blocks[-1]
    load_duration = (t_end - t_start).total_seconds()

    # overall TPS excluding stall time
    stall_time = sum(block_times[j] for j in stall_indices)
    adjusted_duration = load_duration - stall_time
    overall_tps = total_txs / adjusted_duration if adjusted_duration > 0 else 0

    peak_tps = max(steady_tps_values) if steady_tps_values else 0
    median_tps = median(steady_tps_values) if steady_tps_values else 0

    median_bt = median(steady_block_times) if steady_block_times else 0
    fastest_bt = min(steady_block_times) if steady_block_times else 0
    slowest_bt = max(steady_block_times) if steady_block_times else 0

    num_tx_blocks = last_tx_idx - first_tx_idx + 1

    # --- Gas metrics ---
    counted_gas = load_gas[1:] if anchor_is_separate else load_gas
    total_gas_used = sum(gu for gu, _ in counted_gas)
    gas_utilizations = [gu / gl for gu, gl in counted_gas if gl > 0 and gu > 0]
    overall_gps = total_gas_used / adjusted_duration if adjusted_duration > 0 else 0
    peak_gps = max(steady_gps_values) if steady_gps_values else 0
    median_gps = median(steady_gps_values) if steady_gps_values else 0

    # --- Per-tx gas from ETH block data (EVM gas units) ---
    tx_gas_list = [g for g in per_tx_gas_values if g > 0]

    # --- Print TPS section ---
    print("=== TPS ===", file=fp)
    if multi_block:
        print(f"peak_tps {peak_tps:.2f}", file=fp)
        print(f"overall_tps {overall_tps:.2f}", file=fp)
        print(f"median_tps {median_tps:.2f}", file=fp)
        if stall_indices:
            stall_heights = sorted(start + anchor_idx + 1 + j for j in stall_indices)
            print(
                f"stalls_excluded {len(stall_indices)}"
                f" blocks ({stall_time:.1f}s)"
                f" at heights {stall_heights}",
                file=fp,
            )
    else:
        print(
            f"overall_tps N/A (all {total_txs} txs in 1 block; "
            f"increase num_txs for meaningful TPS)",
            file=fp,
        )

    # --- Print Gas Throughput section ---
    print(file=fp)
    print("=== Gas Throughput ===", file=fp)
    print(f"total_gas_used {total_gas_used}", file=fp)
    if multi_block:
        print(f"overall_gps {overall_gps:.0f}", file=fp)
        print(f"peak_gps {peak_gps:.0f}", file=fp)
        print(f"median_gps {median_gps:.0f}", file=fp)
    if gas_utilizations:
        print(
            f"median_gas_utilization" f" {median(gas_utilizations) * 100:.1f}%",
            file=fp,
        )

    # --- Print Per-Tx Gas ---
    if tx_gas_list:
        avg_tx_gas = sum(tx_gas_list) / len(tx_gas_list)
        med_tx_gas = median(tx_gas_list)
        max_tx_gas = max(tx_gas_list)
        min_tx_gas = min(tx_gas_list)
        print(file=fp)
        print("=== Per-Tx Gas ===", file=fp)
        print(f"avg_tx_gas {avg_tx_gas:.0f}", file=fp)
        print(f"median_tx_gas {med_tx_gas:.0f}", file=fp)
        print(f"min_tx_gas {min_tx_gas}", file=fp)
        print(f"max_tx_gas {max_tx_gas}", file=fp)

    # --- Print Block Time ---
    if steady_block_times:
        print(file=fp)
        print("=== Block Time ===", file=fp)
        print(f"median_blocktime {median_bt * 1000:.0f}ms", file=fp)
        print(f"fastest_blocktime {fastest_bt * 1000:.0f}ms", file=fp)
        print(f"slowest_blocktime {slowest_bt * 1000:.0f}ms", file=fp)

    # --- Print Load summary ---
    print(file=fp)
    print("=== Load Summary ===", file=fp)
    duration_str = f"{load_duration:.1f}s"
    if stall_indices:
        duration_str += f" (steady {adjusted_duration:.1f}s, stall {stall_time:.1f}s)"
    print(
        f"load_period blocks {start + first_tx_idx}-{start + last_tx_idx}"
        f" ({num_tx_blocks} blocks, {duration_str})",
        file=fp,
    )
    print(f"total_txs {total_txs}", file=fp)
    if total_counted_txs > 0:
        print(
            f"failed_txs {total_failed_txs}"
            f" ({total_failed_txs / total_counted_txs * 100:.1f}%)",
            file=fp,
        )

    # --- Mempool / Tx-Pool summary ---
    load_mp = mempool_snapshots[anchor_idx : last_tx_idx + 1]
    valid_mp = [n for n, _ in load_mp if n >= 0]
    if valid_mp:
        print(file=fp)
        print("=== Mempool (txpool) ===", file=fp)
        print(f"peak_mempool_txs {max(valid_mp)}", file=fp)
        print(f"median_mempool_txs {median(valid_mp):.0f}", file=fp)
        print(f"end_mempool_txs {valid_mp[-1]}", file=fp)
        valid_mb = [b for _, b in load_mp if b >= 0]
        if valid_mb:
            print(
                f"peak_mempool_bytes {max(valid_mb)}"
                f" ({max(valid_mb) / 1024 / 1024:.1f} MiB)",
                file=fp,
            )

    # --- Prometheus-based metrics (block-stm + consensus) ---
    prom_text = _fetch_prometheus()

    # --- Block-STM from live-collected stm_data ---
    stm_samples = []
    if stm_data and first_tx_idx is not None:
        for idx in range(first_tx_idx, last_tx_idx + 1):
            height = start + idx
            tx_count = blocks[idx][0]
            if tx_count > 0 and height in stm_data:
                executed, validated = stm_data[height]
                stm_samples.append((executed, validated, tx_count))

    if stm_samples:
        print(file=fp)
        print(f"=== Block-STM ({len(stm_samples)} tx-blocks sampled) ===", file=fp)
        total_exec = sum(e for e, _, _ in stm_samples)
        total_valid = sum(v for _, v, _ in stm_samples)
        total_blk_txs = sum(t for _, _, t in stm_samples)
        avg_exec = total_exec / len(stm_samples)
        avg_valid = total_valid / len(stm_samples)
        avg_blk_txs = total_blk_txs / len(stm_samples)
        print(f"avg_stm_executed_txs {avg_exec:.0f}", file=fp)
        print(f"avg_stm_validated_txs {avg_valid:.0f}", file=fp)
        print(f"avg_block_txs {avg_blk_txs:.0f}", file=fp)
        if total_blk_txs > 0:
            reexec_ratio = total_exec / total_blk_txs
            print(
                f"avg_reexecution_ratio {reexec_ratio:.2f}x" f" (1.00x = no conflicts)",
                file=fp,
            )
        if total_exec > 0:
            print(
                f"avg_validation_ratio {total_valid / total_exec:.2f}x",
                file=fp,
            )

    cons = scrape_consensus_metrics(prom_text)
    if cons:
        print(file=fp)
        print("=== Consensus Stage Timing ===", file=fp)

        for key, label in [
            ("abci_finalize_block", "FinalizeBlock (ABCI)"),
            ("abci_commit", "Commit (ABCI)"),
        ]:
            if key in cons:
                avg_s, cnt = cons[key]
                print(f"avg_{key} {avg_s * 1000:.1f}ms ({cnt} samples)", file=fp)

        if "finalize_block_ms" in cons:
            avg_ms, cnt = cons["finalize_block_ms"]
            print(
                f"avg_block_processing {avg_ms:.1f}ms ({cnt} samples)",
                file=fp,
            )

        step_order = [
            "Propose",
            "Prevote",
            "PrevoteWait",
            "Precommit",
            "PrecommitWait",
            "Commit",
            "NewHeight",
            "NewRound",
        ]
        for step in step_order:
            key = f"step_{step}"
            if key in cons:
                avg_s, cnt = cons[key]
                print(
                    f"avg_step_{step.lower()} {avg_s * 1000:.1f}ms" f" ({cnt} samples)",
                    file=fp,
                )

        if "block_interval" in cons:
            avg_s, cnt = cons["block_interval"]
            print(
                f"avg_block_interval {avg_s * 1000:.0f}ms ({cnt} samples)",
                file=fp,
            )
        for key, label in [
            ("quorum_prevote_delay", "quorum_prevote_delay"),
            ("quorum_precommit_delay", "quorum_precommit_delay"),
        ]:
            if key in cons:
                val, _ = cons[key]
                print(f"{label} {val * 1000:.1f}ms", file=fp)
