from datetime import datetime

from .utils import block, block_eth, block_height

# the tps calculation use the average of the last 10 blocks
TPS_WINDOW = 5


def calculate_tps(blocks):
    if len(blocks) < 2:
        return 0

    txs = sum(n for n, _ in blocks[1:])
    _, t1 = blocks[0]
    _, t2 = blocks[-1]
    time_diff = (t2 - t1).total_seconds()
    if time_diff == 0:
        return 0
    return txs / time_diff


def get_block_info_cosmos(height):
    blk = block(height)
    timestamp = datetime.fromisoformat(blk["result"]["block"]["header"]["time"])
    txs = len(blk["result"]["block"]["data"]["txs"])
    return timestamp, txs


def get_block_info_eth(height):
    blk = block_eth(height)
    timestamp = datetime.fromtimestamp(int(blk["timestamp"], 0))
    txs = len(blk["transactions"])
    return timestamp, txs


def dump_block_stats(fp, eth=True):
    """
    dump block stats using web3 json-rpc, which splits batch tx
    """
    tps_list = []
    current = block_height()
    blocks = []
    # skip block 1 whose timestamp is not accurate
    for i in range(2, current + 1):
        if eth:
            timestamp, txs = get_block_info_eth(i)
        else:
            timestamp, txs = get_block_info_cosmos(i)
        blocks.append((txs, timestamp))
        tps = calculate_tps(blocks[-TPS_WINDOW:])
        tps_list.append(tps)
        print("block", i, txs, timestamp, tps, file=fp)
    tps_list.sort(reverse=True)
    print("top_tps", tps_list[:5], file=fp)
