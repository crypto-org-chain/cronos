from datetime import datetime, timezone
from statistics import median

from .utils import LOCAL_JSON_RPC, LOCAL_RPC, block, block_eth, block_height

# sliding window size for per-block TPS calculation
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


def get_block_info_hybrid(height, rpc, json_rpc):
    """
    Use Cosmos RPC for timestamps (sub-second precision) and
    Ethereum JSON-RPC for tx counts (correctly splits batch txs).

    Ethereum timestamps are integer seconds, which introduces up to ±0.5s
    error per block. CometBFT timestamps have nanosecond precision.
    """
    cosmos_blk = block(height, rpc=rpc)
    timestamp = datetime.fromisoformat(cosmos_blk["result"]["block"]["header"]["time"])
    eth_blk = block_eth(height, json_rpc=json_rpc)
    txs = len(eth_blk["transactions"])
    return timestamp, txs


def dump_block_stats(
    fp,
    eth=True,
    json_rpc=LOCAL_JSON_RPC,
    rpc=LOCAL_RPC,
    start: int = 2,
    end: int = None,
):
    """
    Dump per-block stats and TPS metrics.

    When eth=True (default), uses Cosmos RPC for high-precision timestamps
    and Ethereum JSON-RPC for tx counts (which correctly splits batch txs).

    Reports per-block data and a summary section:
    - peak_tps: highest sustained TPS in any sliding window during load
    - load_period: block range and duration of actual transaction activity
    - total_txs: total transactions during load period
    - overall_tps: total_txs / load_duration
    - median_tps: median of per-block sliding-window TPS during load
    - median/fastest/slowest block_time during load
    """
    if end is None:
        end = block_height(rpc)

    blocks = []
    tps_list = []

    # skip block 1 whose timestamp is not accurate
    prev_timestamp = None
    for i in range(start, end + 1):
        if eth:
            timestamp, txs = get_block_info_hybrid(i, rpc, json_rpc)
        else:
            timestamp, txs = get_block_info_cosmos(i, rpc)
        blocks.append((txs, timestamp))
        tps = calculate_tps(blocks[-TPS_WINDOW:])
        tps_list.append(tps)
        if prev_timestamp is not None:
            bt_ms = (timestamp - prev_timestamp).total_seconds() * 1000
            print(f"block {i} {txs} {timestamp} {bt_ms:.0f}ms {tps:.2f}", file=fp)
        else:
            print(f"block {i} {txs} {timestamp} - {tps:.2f}", file=fp)
        prev_timestamp = timestamp

    # --- Summary statistics ---

    # Find load period: first and last blocks containing transactions
    first_tx_idx = None
    last_tx_idx = None
    for idx, (txs, _) in enumerate(blocks):
        if txs > 0:
            if first_tx_idx is None:
                first_tx_idx = idx
            last_tx_idx = idx

    print(file=fp)

    if (
        first_tx_idx is not None
        and last_tx_idx is not None
        and first_tx_idx < last_tx_idx
    ):
        # Use one block before first tx block as time anchor if available,
        # so the first tx block's transactions are fully counted.
        # When first_tx_idx == 0 there is no preceding block in our data;
        # the first tx block doubles as the time anchor and its own txs
        # must still be counted.
        anchor_is_separate = first_tx_idx > 0
        anchor_idx = first_tx_idx - 1 if anchor_is_separate else first_tx_idx
        load_blocks = blocks[anchor_idx : last_tx_idx + 1]

        # Compute per-block sliding-window TPS and block times within load
        load_tps_values = []
        block_times = []
        for j in range(1, len(load_blocks)):
            # Block time: interval between consecutive blocks
            _, t_prev = load_blocks[j - 1]
            _, t_curr = load_blocks[j]
            bt = (t_curr - t_prev).total_seconds()
            block_times.append(bt)

            # Sliding window TPS ending at this block.
            # The window's first element is a true anchor only when it is
            # either the separate anchor block (win_start == 0 and
            # anchor_is_separate) or any interior block (win_start > 0).
            win_start = max(0, j + 1 - TPS_WINDOW)
            window = load_blocks[win_start : j + 1]
            if len(window) >= 2:
                win_has_anchor = anchor_is_separate or win_start > 0
                load_tps_values.append(
                    calculate_tps(window, anchor_is_separate=win_has_anchor)
                )

        # Overall TPS
        counted = load_blocks[1:] if anchor_is_separate else load_blocks
        total_txs = sum(n for n, _ in counted)
        _, t_start = load_blocks[0]
        _, t_end = load_blocks[-1]
        load_duration = (t_end - t_start).total_seconds()
        overall_tps = (
            calculate_tps(load_blocks, anchor_is_separate=anchor_is_separate)
            if load_duration > 0
            else 0
        )

        # TPS stats
        peak_tps = max(load_tps_values) if load_tps_values else 0
        median_tps = median(load_tps_values) if load_tps_values else 0

        # Block time stats
        median_bt = median(block_times) if block_times else 0
        fastest_bt = min(block_times) if block_times else 0
        slowest_bt = max(block_times) if block_times else 0

        num_tx_blocks = last_tx_idx - first_tx_idx + 1
        print(f"peak_tps {peak_tps:.2f}", file=fp)
        print(
            f"load_period blocks {start + first_tx_idx}-{start + last_tx_idx}"
            f" ({num_tx_blocks} blocks, {load_duration:.1f}s)",
            file=fp,
        )
        print(f"total_txs {total_txs}", file=fp)
        print(f"overall_tps {overall_tps:.2f}", file=fp)
        print(f"median_tps {median_tps:.2f}", file=fp)
        print(f"median_blocktime {median_bt * 1000:.0f}ms", file=fp)
        print(f"fastest_blocktime {fastest_bt * 1000:.0f}ms", file=fp)
        print(f"slowest_blocktime {slowest_bt * 1000:.0f}ms", file=fp)
    else:
        print("no_load_period", file=fp)
