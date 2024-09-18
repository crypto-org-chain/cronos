from datetime import datetime

from .utils import block, block_height

# the tps calculation use the average of the last 10 blocks
TPS_WINDOW = 10


def calculate_tps(blocks):
    if len(blocks) < 2:
        return 0

    txs = sum(n for n, _ in blocks)
    _, t1 = blocks[0]
    _, t2 = blocks[-1]
    return txs / (t2 - t1).total_seconds()


def dump_block_stats(fp):
    """
    dump simple statistics for blocks for analysis
    """
    tps_list = []
    current = block_height()
    blocks = []
    for i in range(1, current + 1):
        blk = block(i)
        timestamp = datetime.fromisoformat(blk["result"]["block"]["header"]["time"])
        txs = len(blk["result"]["block"]["data"]["txs"])
        blocks.append((txs, timestamp))
        tps = calculate_tps(blocks[-TPS_WINDOW:])
        tps_list.append(tps)
        print("block", i, txs, timestamp, tps, file=fp)
    tps_list.sort(reverse=True)
    print("top_tps", tps_list[:5], file=fp)
