"""TPS/GPS window analysis: sliding-window TPS, percentiles, and the
load-window summary shared by dump_block_stats and dump_eth_block_stats.
"""

from statistics import median, quantiles

TPS_WINDOW = 10


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


def _best_window_tps(load_blocks, anchor_is_separate, window_blocks=30):
    """Best average TPS over any window_blocks consecutive blocks within the
    load period - the steady-state rate once warm-up/JIT/cache effects have
    settled, as opposed to overall_tps which is dragged down by the warm-up
    blocks it's averaged together with.

    cum[i] is the tx count counted "up to and including" load_blocks[i]; when
    anchor_is_separate, load_blocks[0] is a pure time anchor so cum[0] = 0 and
    its own txs never enter any window's numerator. A window [left, right] is
    scored as (cum[right] - cum[left]) / (ts[right] - ts[left]) - left's txs
    are excluded the same way the anchor's are, since left is the window's own
    time anchor. window_blocks counts blocks, not intervals, so a window
    spans window_blocks - 1 intervals (left..right inclusive).

    A window straddling a stalled block scores lower and simply loses to a
    clean window elsewhere - no separate stall-exclusion needed here.
    """
    n = len(load_blocks)
    if n < 2:
        return 0

    cum = [0] * n
    start_idx = 1 if anchor_is_separate else 0
    for i in range(start_idx, n):
        cum[i] = cum[i - 1] + load_blocks[i][0] if i > 0 else load_blocks[i][0]

    timestamps = [t for _, t in load_blocks]
    counted_blocks = n - (1 if anchor_is_separate else 0)
    if counted_blocks < window_blocks:
        span = (timestamps[-1] - timestamps[0]).total_seconds()
        return cum[-1] / span if span > 0 else 0

    best = 0
    for right in range(window_blocks - 1, n):
        left = right - (window_blocks - 1)
        span = (timestamps[right] - timestamps[left]).total_seconds()
        if span > 0:
            tps = (cum[right] - cum[left]) / span
            best = max(best, tps)
    return best


def _percentile(values, pct):
    """Linear-interpolated percentile (0-100) of a list of numbers."""
    if not values:
        return 0
    s = sorted(values)
    if len(s) == 1:
        return s[0]
    k = (len(s) - 1) * (pct / 100)
    f = int(k)
    c = min(f + 1, len(s) - 1)
    if f == c:
        return s[f]
    return s[f] * (c - k) + s[c] * (k - f)


def _analyze_load_window(
    blocks,
    gas_data,
    per_tx_gas_values,
    total_failed_txs=0,
    total_counted_txs=0,
    stall_mult=5,
    stall_min_seconds=1.0,
):
    """Compute the summary statistics shared by dump_block_stats and
    dump_eth_block_stats from a queried block range.

    blocks: list of (tx_count, timestamp) for every queried height.
    gas_data: list of (gas_used, gas_limit), parallel to blocks.
    per_tx_gas_values: (per-tx gas, block gas limit) for blocks with
        tx_count > 0.

    stall_min_seconds: absolute floor alongside the 5xQ1 relative rule - a
        block time under this is never a stall, even if it is 5x the
        (possibly tiny) 25th-percentile baseline. Without it, a network
        with sub-second, low-variance block times flags routine jitter as a
        "stall" the moment it is 5x Q1, which for e.g. a 40ms Q1 is 200ms.

    Returns None if no block in the range had any transactions (the
    "no_load_period" case), else a dict consumed by
    _print_load_summary_sections and the callers' own extra sections
    (mempool/failed-tx/block-stm/consensus).
    """
    first_tx_idx = None
    last_tx_idx = None
    for idx, (txs, _) in enumerate(blocks):
        if txs > 0:
            if first_tx_idx is None:
                first_tx_idx = idx
            last_tx_idx = idx

    if first_tx_idx is None:
        return None

    multi_block = first_tx_idx < last_tx_idx
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
        else:
            # Keep index alignment with block_times so the stall filter below
            # drops the right samples; a zero-length interval has no rate.
            load_gps_values.append(None)

        win_start = max(0, j + 1 - TPS_WINDOW)
        window = load_blocks[win_start : j + 1]
        if len(window) >= 2:
            win_has_anchor = anchor_is_separate or win_start > 0
            load_tps_values.append(
                calculate_tps(window, anchor_is_separate=win_has_anchor)
            )

    # --- Detect stalled blocks ---
    # Use the 25th-percentile block time as the "normal" baseline.
    # Blocks slower than stall_mult × baseline are stalls (e.g. tx-flood
    # overwhelming the proposer) and are excluded from timing summaries.
    # stall_min_seconds guards against flagging routine jitter on a
    # low-variance, sub-second network as a stall.
    stall_indices = set()
    if len(block_times) >= 4:
        q1 = quantiles(block_times, n=4)[0]
        stall_threshold = q1 * stall_mult
        for j, bt in enumerate(block_times):
            if bt > stall_threshold and bt > stall_min_seconds:
                stall_indices.add(j)

    # --- Classify each stall ---
    # ramp-artifact: among the first few intervals of the window, where
    # warm-up effects (JIT, cold caches) commonly produce one slow block
    # that has nothing to do with steady-state load.
    # wedge: the last interval in the window - the query range ended right
    # after this block because no further height was ever reached, not
    # because the caller happened to stop looking there.
    # slow-block: everything else - a full, slow block that the chain
    # nonetheless recovered from and kept committing past.
    _RAMP_ARTIFACT_WINDOW = 3
    last_interval_idx = len(block_times) - 1
    stall_kinds = {}
    for j in stall_indices:
        if j == last_interval_idx:
            stall_kinds[j] = "wedge"
        elif j < _RAMP_ARTIFACT_WINDOW:
            stall_kinds[j] = "ramp-artifact"
        else:
            stall_kinds[j] = "slow-block"

    steady_block_times = [
        bt for j, bt in enumerate(block_times) if j not in stall_indices
    ]
    steady_tps_values = [
        v for j, v in enumerate(load_tps_values) if j not in stall_indices
    ]
    steady_gps_values = [
        v
        for j, v in enumerate(load_gps_values)
        if j not in stall_indices and v is not None
    ]

    counted = load_blocks[1:] if anchor_is_separate else load_blocks
    total_txs = sum(n for n, _ in counted)
    _, t_start = load_blocks[0]
    _, t_end = load_blocks[-1]
    load_duration = (t_end - t_start).total_seconds()

    # overall TPS excluding stalls. block_times[j] is the interval ending at
    # load_blocks[j + 1], so that block's txs/gas are what get excluded along
    # with its time — numerator and denominator stay consistent (steady-only).
    stall_time = sum(block_times[j] for j in stall_indices)
    stall_txs = sum(load_blocks[j + 1][0] for j in stall_indices)
    stall_gas = sum(load_gas[j + 1][0] for j in stall_indices)
    adjusted_duration = load_duration - stall_time
    steady_txs = total_txs - stall_txs
    overall_tps = steady_txs / adjusted_duration if adjusted_duration > 0 else 0

    peak_tps = max(steady_tps_values) if steady_tps_values else 0
    median_tps = median(steady_tps_values) if steady_tps_values else 0
    best_30block_tps = _best_window_tps(load_blocks, anchor_is_separate)

    median_bt = median(steady_block_times) if steady_block_times else 0
    fastest_bt = min(steady_block_times) if steady_block_times else 0
    slowest_bt = max(steady_block_times) if steady_block_times else 0
    p95_bt = _percentile(steady_block_times, 95)
    p99_bt = _percentile(steady_block_times, 99)

    num_tx_blocks = last_tx_idx - first_tx_idx + 1

    # --- Gas metrics ---
    counted_gas = load_gas[1:] if anchor_is_separate else load_gas
    total_gas_used = sum(gu for gu, _ in counted_gas)
    # Only gl == 0 is dropped: that means the block's gas was never measured
    # (an unreachable block reads as (0, 0)). A block with a gas limit and zero
    # gas used is an empty block, exactly the underutilization the saturation
    # gate exists to catch, so it stays in the sample.
    gas_utilizations = [gu / gl for gu, gl in counted_gas if gl > 0]
    steady_gas_used = total_gas_used - stall_gas
    overall_gps = steady_gas_used / adjusted_duration if adjusted_duration > 0 else 0
    peak_gps = max(steady_gps_values) if steady_gps_values else 0
    median_gps = median(steady_gps_values) if steady_gps_values else 0

    # --- Per-tx gas from ETH block data (EVM gas units) ---
    # Keyed off the block's gas limit for the same reason as gas_utilizations
    # above: gl == 0 means the block's gas was never measured, while a measured
    # block with zero gas used is real data.
    tx_gas_list = [gu for gu, gl in per_tx_gas_values if gl > 0]

    return {
        "first_tx_idx": first_tx_idx,
        "last_tx_idx": last_tx_idx,
        "anchor_idx": anchor_idx,
        "multi_block": multi_block,
        "num_tx_blocks": num_tx_blocks,
        "total_txs": total_txs,
        "load_duration": load_duration,
        "raw_avg_tps": total_txs / load_duration if load_duration > 0 else 0,
        "overall_tps": overall_tps,
        "peak_tps": peak_tps,
        "median_tps": median_tps,
        "best_30block_tps": best_30block_tps,
        "stall_indices": stall_indices,
        "stall_time": stall_time,
        "adjusted_duration": adjusted_duration,
        # heights of stalled blocks, as offsets from the queried `start`.
        "stall_height_offsets": sorted(anchor_idx + 1 + j for j in stall_indices),
        # {height_offset: "wedge"|"ramp-artifact"|"slow-block"} - see the
        # classification block above for what each kind means.
        "stall_kinds": {anchor_idx + 1 + j: kind for j, kind in stall_kinds.items()},
        "total_gas_used": total_gas_used,
        "gas_utilizations": gas_utilizations,
        "overall_gps": overall_gps,
        "peak_gps": peak_gps,
        "median_gps": median_gps,
        "tx_gas_list": tx_gas_list,
        "steady_block_times": steady_block_times,
        "median_bt": median_bt,
        "fastest_bt": fastest_bt,
        "slowest_bt": slowest_bt,
        "p95_bt": p95_bt,
        "p99_bt": p99_bt,
        "total_failed_txs": total_failed_txs,
        "total_counted_txs": total_counted_txs,
    }
