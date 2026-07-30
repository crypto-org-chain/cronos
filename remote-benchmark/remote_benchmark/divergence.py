"""Multi-node state-divergence checks for a devnet run.

A devnet node can silently diverge — a stuck peer, an app-hash mismatch after
an upgrade — while `/status` still returns 200 for every node. Stats sourced
from `cfg.primary` alone can't catch that, so this module checks the whole
endpoint roster: how far behind the slowest node is, and whether every node
that reached a given height agreed on its app hash.
"""

from .utils import block, block_height


def collect_heights(endpoints):
    """Return {endpoint.name: height}, or None for an endpoint that errored."""
    heights = {}
    for endpoint in endpoints:
        try:
            heights[endpoint.name] = block_height(endpoint.rpc)
        except Exception:
            heights[endpoint.name] = None
    return heights


def height_skew(heights):
    """Max height gap among reachable nodes. None if fewer than 2 are reachable."""
    reachable = [height for height in heights.values() if height is not None]
    if len(reachable) < 2:
        return None
    return max(reachable) - min(reachable)


def app_hash_at(height, rpc):
    return block(height, rpc)["result"]["block"]["header"]["app_hash"]


def check_app_hash_agreement(endpoints, start, end):
    """Compare app_hash across every endpoint that reached each height in
    [start, end].

    Returns one {height, hashes} entry per height where the reachable nodes
    disagree. A height no node has reached yet, or a node that errored on a
    given height, is simply left out of that height's `hashes` rather than
    treated as a mismatch — the check only flags nodes that actually replied
    with different app hashes for the same height.
    """
    divergences = []
    for height in range(start, end + 1):
        hashes = {}
        for endpoint in endpoints:
            try:
                hashes[endpoint.name] = app_hash_at(height, endpoint.rpc)
            except Exception:
                continue
        if len(set(hashes.values())) > 1:
            divergences.append({"height": height, "hashes": hashes})
    return divergences
