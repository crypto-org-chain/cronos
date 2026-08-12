import logging
from concurrent.futures import ThreadPoolExecutor
from datetime import datetime, timezone
from statistics import median

from . import resources
from .cometbft_metrics import (
    scrape_blockstm_metrics,
    scrape_consensus_health,
    scrape_consensus_health_raw,
    scrape_consensus_metrics,
    scrape_consensus_raw,
    scrape_per_validator_metrics,
)
from .promtext import (
    fetch_prometheus_text as _fetch_prometheus,
    labeled_metric_by as _labeled_metric_by,
    parse_histogram_sum_count as _parse_histogram_sum_count,
    parse_label_block as _parse_label_block,
    parse_labeled_metric as _parse_labeled_metric,
)
from .utils import (
    block,
    block_eth,
    block_height,
    block_results,
    blockchain_range,
    eth_block_number,
    mempool_status,
)
from .window import TPS_WINDOW, _analyze_load_window, _percentile, calculate_tps

log = logging.getLogger(__name__)

# Number of concurrent /block_results fetches for the failed-tx detail pass -
# same order of magnitude as transaction.py's send concurrency, bounded so a
# huge load window doesn't flood the SSH tunnel with simultaneous requests.
FAILED_TX_FETCH_WORKERS = 16


def get_block_info_cosmos(height, rpc):
    blk = block(height, rpc)
    timestamp = datetime.fromisoformat(blk["result"]["block"]["header"]["time"])
    txs = len(blk["result"]["block"]["data"]["txs"])
    return timestamp, txs


def get_block_info_eth(height, json_rpc):
    blk = block_eth(height, json_rpc)
    timestamp = datetime.fromtimestamp(int(blk["timestamp"], 0), tz=timezone.utc)
    txs = len(blk["transactions"])
    return timestamp, txs


def _extract_gas(eth_blk):
    """Extract (gas_used, gas_limit) from an already-fetched eth block dict."""
    gas_used = int(eth_blk.get("gasUsed", "0x0"), 16)
    gas_limit = int(eth_blk.get("gasLimit", "0x0"), 16)
    return gas_used, gas_limit


def _get_failed_tx_count(height, rpc):
    """Number of failed txs from CometBFT block_results, or None when the
    query fails — a zero there would read as "no failures" and let the
    saturation gate pass on data that was never measured."""
    try:
        res = block_results(height, rpc)
        tx_results = res.get("result", {}).get("txs_results") or []
        return sum(1 for r in tx_results if int(r.get("code", 0)) != 0)
    except Exception:
        log.debug("block_results unavailable for height %d", height, exc_info=True)
        return None


def _get_block_gas_and_txs(height, json_rpc):
    """Ethereum JSON-RPC-only fetch of tx count and gas data for one height.
    Timestamps come from a separate, chunked /blockchain fetch instead of a
    per-height Cosmos /block call - see _collect_block_range.
    """
    eth_blk = block_eth(height, json_rpc)
    txs = len(eth_blk["transactions"])
    gas_used, gas_limit = _extract_gas(eth_blk)
    return txs, gas_used, gas_limit


def get_block_info_eth_full(height, json_rpc):
    """
    Use plain Ethereum JSON-RPC only (no Cosmos RPC) for timestamp, tx count,
    and gas data. This is the eth-mode analog of _get_block_gas_and_txs, for
    nodes (e.g. Anvil) with no CometBFT/Cosmos RPC.

    Returns (timestamp, tx_count, gas_used, gas_limit).
    """
    eth_blk = block_eth(height, json_rpc)
    timestamp = datetime.fromtimestamp(int(eth_blk["timestamp"], 16), tz=timezone.utc)
    txs = len(eth_blk["transactions"])
    gas_used, gas_limit = _extract_gas(eth_blk)
    return timestamp, txs, gas_used, gas_limit


def _print_load_summary_sections(fp, start, summary):
    """Print the TPS / Gas Throughput / Per-Tx Gas / Block Time / Load
    Summary sections shared by dump_block_stats and dump_eth_block_stats."""
    print("=== TPS ===", file=fp)
    if summary["multi_block"]:
        print(f"peak_tps {summary['peak_tps']:.2f}", file=fp)
        print(f"overall_tps {summary['overall_tps']:.2f}", file=fp)
        # Same window as overall_tps but without excluding stalled blocks/txs -
        # total_txs / load_duration, i.e. the naive sum(txs)/(t_end - t_start).
        # Useful as a sanity cross-check against overall_tps; the gap between
        # the two is exactly how much the stall exclusion is buying you.
        print(f"raw_avg_tps {summary['raw_avg_tps']:.2f}", file=fp)
        print(f"median_tps {summary['median_tps']:.2f}", file=fp)
        if summary["stall_indices"]:
            stall_heights = [start + off for off in summary["stall_height_offsets"]]
            print(
                f"stalls_excluded {len(summary['stall_indices'])}"
                f" blocks ({summary['stall_time']:.1f}s)"
                f" at heights {stall_heights}",
                file=fp,
            )
    else:
        print(
            f"overall_tps N/A (all {summary['total_txs']} txs in 1 block; "
            f"increase num_txs for meaningful TPS)",
            file=fp,
        )

    print(file=fp)
    print("=== Gas Throughput ===", file=fp)
    print(f"total_gas_used {summary['total_gas_used']}", file=fp)
    if summary["multi_block"]:
        print(f"overall_gps {summary['overall_gps']:.0f}", file=fp)
        print(f"peak_gps {summary['peak_gps']:.0f}", file=fp)
        print(f"median_gps {summary['median_gps']:.0f}", file=fp)
    if summary["gas_utilizations"]:
        print(
            f"median_gas_utilization"
            f" {median(summary['gas_utilizations']) * 100:.1f}%",
            file=fp,
        )

    if summary["tx_gas_list"]:
        tx_gas_list = summary["tx_gas_list"]
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

    if summary["steady_block_times"]:
        print(file=fp)
        print("=== Block Time ===", file=fp)
        print(f"median_blocktime {summary['median_bt'] * 1000:.0f}ms", file=fp)
        print(f"fastest_blocktime {summary['fastest_bt'] * 1000:.0f}ms", file=fp)
        print(f"slowest_blocktime {summary['slowest_bt'] * 1000:.0f}ms", file=fp)
        print(f"p95_blocktime {summary['p95_bt'] * 1000:.0f}ms", file=fp)
        print(f"p99_blocktime {summary['p99_bt'] * 1000:.0f}ms", file=fp)

    print(file=fp)
    print("=== Load Summary ===", file=fp)
    duration_str = f"{summary['load_duration']:.1f}s"
    if summary["stall_indices"]:
        duration_str += (
            f" (steady {summary['adjusted_duration']:.1f}s,"
            f" stall {summary['stall_time']:.1f}s)"
        )
    print(
        f"load_period blocks {start + summary['first_tx_idx']}"
        f"-{start + summary['last_tx_idx']}"
        f" ({summary['num_tx_blocks']} blocks, {duration_str})",
        file=fp,
    )
    print(f"total_txs {summary['total_txs']}", file=fp)
    if summary["total_counted_txs"] > 0:
        print(
            f"failed_txs {summary['total_failed_txs']}"
            f" ({summary['total_failed_txs'] / summary['total_counted_txs'] * 100:.1f}%)",
            file=fp,
        )


def _print_block_line(fp, i, txs, gas_used, timestamp, prev_timestamp, mp_str=""):
    if prev_timestamp is not None:
        bt = (timestamp - prev_timestamp).total_seconds()
        bt_ms = bt * 1000
        # Instantaneous per-block TPS: this block's txs over its own block
        # time. Avoids the sliding-window artifact where an early stall
        # block distorts the rate of later blocks as it moves through the
        # window. See dump_block_stats summary for windowed peak/median.
        tps = txs / bt if bt > 0 else 0
        gas_str = f" gas={gas_used}" if gas_used > 0 else ""
        print(
            f"block {i} txs={txs}{gas_str}"
            f" {timestamp.isoformat()} {bt_ms:.0f}ms tps={tps:.2f}{mp_str}",
            file=fp,
        )
    else:
        print(
            f"block {i} txs={txs} {timestamp.isoformat()} - tps=0.00{mp_str}",
            file=fp,
        )


def _collect_block_range(rpc, json_rpc, eth, start, end, mempool_data=None):
    """Fetch per-block timestamp/tx/gas/failed-tx/mempool data for
    [start, end]. Pure data collection — no printing.

    mempool_data: optional dict {block_height: (peak_n_txs, peak_n_bytes)}
        collected during the load period by a MempoolMonitor. When omitted,
        each height's mempool is queried live as the range is walked.

    Returns a dict with parallel lists blocks/gas_data/mempool_snapshots
    (one entry per height in [start, end]), per_tx_gas_values (one entry per
    height with tx_count > 0), and the failed-tx totals.
    """
    blocks = []
    gas_data = []
    per_tx_gas_values = []
    total_failed_txs = 0
    total_counted_txs = 0
    mempool_snapshots = []

    # /blockchain returns up to BLOCKCHAIN_PAGE_SIZE block_metas per call
    # (timestamp + tx count), so this replaces one /block call per height
    # with a handful of calls for the whole range.
    metas = blockchain_range(start, end, rpc)

    if eth:
        block_info = {
            i: (datetime.fromisoformat(metas[i][1]), *_get_block_gas_and_txs(i, json_rpc))
            for i in range(start, end + 1)
        }
    else:
        block_info = {
            i: (datetime.fromisoformat(metas[i][1]), metas[i][0], 0, 0)
            for i in range(start, end + 1)
        }
    heights_with_txs = [i for i in range(start, end + 1) if block_info[i][1] > 0]
    with ThreadPoolExecutor(max_workers=FAILED_TX_FETCH_WORKERS) as pool:
        failed_counts = dict(
            zip(
                heights_with_txs,
                pool.map(lambda h: _get_failed_tx_count(h, rpc), heights_with_txs),
            )
        )

    # mempool_status has no historical query - it always reports the current
    # live snapshot, so when mempool_data wasn't captured during the run
    # (the post-hoc `stats` command), querying it per height in the loop
    # below would just fetch the same value len(range) times. Fetch once.
    live_mempool_snapshot = None
    if mempool_data is None:
        try:
            live_mempool_snapshot = mempool_status(rpc)
        except Exception:
            live_mempool_snapshot = (-1, -1)

    for i in range(start, end + 1):
        timestamp, txs, gas_used, gas_limit = block_info[i]

        if txs > 0:
            failed = failed_counts[i]
            # A block whose failure count couldn't be read contributes to
            # neither side of the ratio, so total_counted_txs stays 0 when
            # nothing was measurable and the failed-tx gate reports no data.
            if failed is not None:
                total_failed_txs += failed
                total_counted_txs += txs
            per_tx_gas_values.append((gas_used // txs, gas_limit))
        gas_data.append((gas_used, gas_limit))
        blocks.append((txs, timestamp))

        if mempool_data is not None:
            mp_txs, mp_bytes = mempool_data.get(i, (-1, -1))
        else:
            mp_txs, mp_bytes = live_mempool_snapshot
        mempool_snapshots.append((mp_txs, mp_bytes))

    return {
        "blocks": blocks,
        "gas_data": gas_data,
        "per_tx_gas_values": per_tx_gas_values,
        "total_failed_txs": total_failed_txs,
        "total_counted_txs": total_counted_txs,
        "mempool_snapshots": mempool_snapshots,
    }


def _print_blocks(fp, start, blocks, gas_data, mempool_snapshots):
    prev_timestamp = None
    for offset, ((txs, timestamp), (gas_used, _)) in enumerate(zip(blocks, gas_data)):
        mp_txs, _ = mempool_snapshots[offset]
        mp_str = f" mempool={mp_txs}" if mp_txs >= 0 else ""
        _print_block_line(fp, start + offset, txs, gas_used, timestamp, prev_timestamp, mp_str)
        prev_timestamp = timestamp


def _print_mempool(fp, summary, mempool_snapshots):
    """Print the Mempool (txpool) section and record mempool_min_pending on
    the summary.

    Excludes both edges of the window. The leading anchor block predates any
    load tx, and the trailing block is where the last tx commits and the load
    generator has already stopped, so both snapshot a drained mempool and
    would drag mempool_min_pending to 0 and trip the saturation gate on a
    healthy run. A single-tx-block window has no interior left, so it keeps
    that one block rather than measuring nothing.
    """
    first_mp_idx = summary["first_tx_idx"]
    last_mp_idx = summary["last_tx_idx"]
    if last_mp_idx > first_mp_idx:
        last_mp_idx -= 1
    load_mp = mempool_snapshots[first_mp_idx : last_mp_idx + 1]
    valid_mp = [n for n, _ in load_mp if n >= 0]
    summary["mempool_min_pending"] = min(valid_mp) if valid_mp else None
    if not valid_mp:
        return

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


def _print_blockstm(fp, summary, start, blocks, stm_data):
    """Print the Block-STM section from live-collected stm_data, and record
    avg_reexecution_ratio/avg_validation_ratio on the summary."""
    stm_samples = []
    if stm_data:
        for idx in range(summary["first_tx_idx"], summary["last_tx_idx"] + 1):
            height = start + idx
            tx_count = blocks[idx][0]
            if tx_count > 0 and height in stm_data:
                executed, validated = stm_data[height]
                stm_samples.append((executed, validated, tx_count))

    summary["avg_reexecution_ratio"] = None
    summary["avg_validation_ratio"] = None
    if not stm_samples:
        return

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
        summary["avg_reexecution_ratio"] = reexec_ratio
        print(
            f"avg_reexecution_ratio {reexec_ratio:.2f}x (1.00x = no conflicts)",
            file=fp,
        )
    if total_exec > 0:
        validation_ratio = total_valid / total_exec
        summary["avg_validation_ratio"] = validation_ratio
        print(f"avg_validation_ratio {validation_ratio:.2f}x", file=fp)


def _print_consensus_timing(fp, prom_text, consensus_baseline, scope):
    """Print the Consensus Stage Timing section."""
    cons = scrape_consensus_metrics(prom_text, baseline=consensus_baseline)
    if not cons:
        return

    print(file=fp)
    print(f"=== Consensus Stage Timing ({scope}) ===", file=fp)

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


def _print_consensus_health(fp, prom_text, consensus_health_baseline, summary, scope):
    """Print the Consensus Health section and record missing/byzantine
    validator counts on the summary."""
    health = scrape_consensus_health(prom_text, baseline=consensus_health_baseline)
    # Carried on the summary, not just printed: a validator set that lost a
    # member or reported byzantine behaviour is a consensus finding the
    # caller has to be able to gate on.
    summary["missing_validators"] = health["missing_validators"]
    summary["byzantine_validators"] = health["byzantine_validators"]

    print(file=fp)
    print(f"=== Consensus Health ({scope}) ===", file=fp)
    print(f"round_increments {health['round_increments']:.0f}", file=fp)
    if health["current_round"] is not None:
        print(f"current_round {health['current_round']:.0f}", file=fp)
    print(f"rejected_proposals {health['rejected_proposals']:.0f}", file=fp)
    print(f"late_votes {health['late_votes']:.0f}", file=fp)
    print(f"duplicate_block_parts {health['duplicate_block_parts']:.0f}", file=fp)
    print(f"duplicate_votes {health['duplicate_votes']:.0f}", file=fp)
    print(
        f"block_gossip_parts_mismatched {health['block_gossip_parts_mismatched']:.0f}",
        file=fp,
    )
    if health["missing_validators"] is not None:
        print(f"missing_validators {health['missing_validators']:.0f}", file=fp)
    if health["byzantine_validators"] is not None:
        print(f"byzantine_validators {health['byzantine_validators']:.0f}", file=fp)


def _print_per_validator(fp, prom_text):
    """Print the Per-Validator section."""
    per_validator = scrape_per_validator_metrics(prom_text)
    if not per_validator:
        return

    print(file=fp)
    print("=== Per-Validator ===", file=fp)
    for addr, stats in sorted(per_validator.items()):
        missed = stats["missed_blocks"]
        signed = stats["last_signed_height"]
        power = stats["power"]
        print(
            f"{addr}"
            f" missed_blocks={missed if missed is not None else 'N/A'}"
            f" last_signed_height={signed if signed is not None else 'N/A'}"
            f" power={power if power is not None else 'N/A'}",
            file=fp,
        )


def _print_resources(fp, telemetry, node_exporter, prom_text, disk_net_baseline):
    """Print the Resources section: Go runtime gauges (from telemetry) and
    disk/network I/O deltas (from node_exporter)."""
    if not (telemetry or node_exporter):
        return

    print(file=fp)
    print("=== Resources ===", file=fp)
    if telemetry:
        go = resources.scrape_go_runtime(prom_text)
        if go["rss_bytes"] is not None:
            print(f"rss_bytes {go['rss_bytes']:.0f}", file=fp)
        if go["goroutines"] is not None:
            print(f"goroutines {go['goroutines']:.0f}", file=fp)
        if go["heap_alloc_bytes"] is not None:
            print(f"heap_alloc_bytes {go['heap_alloc_bytes']:.0f}", file=fp)
    if node_exporter:
        disk_net = resources.scrape_disk_net(
            resources.fetch_node_exporter(node_exporter), baseline=disk_net_baseline
        )
        if disk_net is None:
            print("disk_net N/A (node_exporter scrape returned no counters)", file=fp)
        else:
            # Scoped independently of the consensus baseline: the disk/net
            # baseline scrape can fail on its own, leaving lifetime totals.
            disk_scope = "load period" if disk_net_baseline else "node lifetime"
            print(f"disk_read_bytes {disk_net['disk_read_bytes']:.0f} ({disk_scope})", file=fp)
            print(f"disk_written_bytes {disk_net['disk_written_bytes']:.0f} ({disk_scope})", file=fp)
            print(
                f"network_receive_bytes {disk_net['network_receive_bytes']:.0f} ({disk_scope})",
                file=fp,
            )
            print(
                f"network_transmit_bytes {disk_net['network_transmit_bytes']:.0f} ({disk_scope})",
                file=fp,
            )


def dump_block_stats(
    fp,
    rpc: str | list[str],
    json_rpc: str | list[str],
    eth: bool = True,
    telemetry: str = None,
    start: int = 2,
    end: int = None,
    mempool_data: dict = None,
    stm_data: dict = None,
    consensus_baseline: dict = None,
    consensus_health_baseline: dict = None,
    node_exporter: str = None,
    disk_net_baseline: dict = None,
):
    """
    Dump per-block stats and summary metrics.

    Reports per-block data and a summary section with:
    - TPS: peak, overall, median
    - Gas throughput: GPS (gas per second), peak GPS
    - Gas utilization: median gas_used / gas_limit ratio
    - Per-tx gas: avg, median, max
    - Block time: median, fastest, slowest, p95, p99
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

    telemetry: optional Prometheus telemetry base URL (e.g.
        http://host:26660). When omitted, block-stm/consensus sections are
        skipped since that data is unavailable.
    """
    if end is None:
        end = block_height(rpc)

    collected = _collect_block_range(rpc, json_rpc, eth, start, end, mempool_data)
    blocks = collected["blocks"]
    gas_data = collected["gas_data"]
    mempool_snapshots = collected["mempool_snapshots"]

    _print_blocks(fp, start, blocks, gas_data, mempool_snapshots)
    print(file=fp)

    summary = _analyze_load_window(
        blocks,
        gas_data,
        collected["per_tx_gas_values"],
        total_failed_txs=collected["total_failed_txs"],
        total_counted_txs=collected["total_counted_txs"],
    )
    if summary is None:
        print("no_load_period", file=fp)
        return None

    _print_load_summary_sections(fp, start, summary)
    _print_mempool(fp, summary, mempool_snapshots)

    # --- Prometheus-based metrics (block-stm + consensus) ---
    prom_text = _fetch_prometheus(telemetry)

    _print_blockstm(fp, summary, start, blocks, stm_data)

    scope = "load period" if consensus_baseline else "node lifetime"
    _print_consensus_timing(fp, prom_text, consensus_baseline, scope)

    summary["missing_validators"] = None
    summary["byzantine_validators"] = None
    if telemetry:
        _print_consensus_health(fp, prom_text, consensus_health_baseline, summary, scope)
        _print_per_validator(fp, prom_text)

    _print_resources(fp, telemetry, node_exporter, prom_text, disk_net_baseline)

    return summary


def dump_eth_block_stats(fp, json_rpc: str | list[str], start: int = 2, end: int = None):
    """
    Dump per-block stats and summary metrics using plain Ethereum JSON-RPC
    only (no Cosmos/CometBFT RPC, no Prometheus telemetry) — for nodes like
    Anvil that don't expose those. Covers TPS, gas throughput, per-tx gas,
    block time, and a load summary; drops the mempool, failed-tx, Block-STM,
    and consensus-timing sections since those need CometBFT/Cosmos SDK
    telemetry unavailable on plain EVM nodes.
    """
    if end is None:
        end = eth_block_number(json_rpc)

    blocks = []
    gas_data = []
    per_tx_gas_values = []

    prev_timestamp = None
    for i in range(start, end + 1):
        timestamp, txs, gas_used, gas_limit = get_block_info_eth_full(i, json_rpc)

        if txs > 0:
            per_tx_gas_values.append((gas_used // txs, gas_limit))
        gas_data.append((gas_used, gas_limit))
        blocks.append((txs, timestamp))

        _print_block_line(fp, i, txs, gas_used, timestamp, prev_timestamp)
        prev_timestamp = timestamp

    print(file=fp)

    summary = _analyze_load_window(blocks, gas_data, per_tx_gas_values)
    if summary is None:
        print("no_load_period", file=fp)
        return None

    _print_load_summary_sections(fp, start, summary)
    return summary
