"""Multi-node state-divergence checks for a devnet run.

A devnet node can silently diverge — a stuck peer, an app-hash mismatch after
an upgrade — while `/status` still returns 200 for every node. Stats sourced
from `cfg.primary` alone can't catch that, so this module checks the whole
endpoint roster: how far behind the slowest node is, and whether every node
agreed on the app hash it computed for itself.
"""

import time

from .utils import abci_info, block_height

# Enough forward samples to see every node commit past a shared starting tip.
DIVERGENCE_BLOCKS = 3
DIVERGENCE_INTERVAL_S = 0.5
DIVERGENCE_TIMEOUT_S = 60


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


def abci_app_hash(rpc):
    """The (height, app_hash) a node computed for itself, from the app's own
    last commit ID.

    A block header's app_hash is consensus data, byte-identical on every node
    that manages to commit the block at all, so comparing it across nodes can
    never surface a divergence. This is a per-node measurement and can actually
    differ. Returns height 0 while the node has committed nothing.
    """
    response = abci_info(rpc)["result"]["response"]
    return int(response.get("last_block_height", 0) or 0), response.get(
        "last_block_app_hash", ""
    )


def check_app_hash_agreement(
    endpoints,
    blocks=DIVERGENCE_BLOCKS,
    interval=DIVERGENCE_INTERVAL_S,
    timeout=DIVERGENCE_TIMEOUT_S,
):
    """Sample every node's own computed app hash while `blocks` further blocks
    commit, then report every height where two nodes disagreed.

    Sampling has to run forward in time: a node only exposes the hash it
    computed for its current tip, so past heights cannot be asked for after the
    fact. A node whose execution diverged panics with "wrong Block.Header.AppHash"
    and stops answering, so silence from any node is reported as a divergence
    symptom, as is a chain that never advances `blocks` heights and a window in
    which no height was ever reported by two nodes — none of those verified
    agreement, and an empty result has to mean "checked and agreed".

    Returns a list of entries, each carrying a `reason`, a `hashes` mismatch, or
    both; empty means agreement was actually observed. Every entry carries a
    `kind`: "diverged" for a confirmed mismatch, "unverified" for an outcome that
    only means agreement could not be established.
    """
    divergences = []
    if len(endpoints) < 2:
        return [
            {
                "kind": "unverified",
                "reason": f"need two endpoints to compare app hashes, got "
                f"{[endpoint.name for endpoint in endpoints]}",
            }
        ]

    seen = {}  # height -> {name: app_hash}
    tips = {}  # name -> highest height it reported
    empty_hash = set()  # names already reported for an absent app hash
    start_tip = None
    deadline = time.monotonic() + timeout
    while True:
        for endpoint in endpoints:
            try:
                height, app_hash = abci_app_hash(endpoint.rpc)
            except Exception:
                continue
            if height == 0:
                continue
            tips[endpoint.name] = max(tips.get(endpoint.name, 0), height)
            # `last_block_app_hash` is omitempty, so an absent or renamed field
            # reads as "" on every node: they would all compare equal having
            # compared nothing. A committed height always has a 32-byte commit
            # hash, so emptiness is a schema/build defect rather than a transient
            # blip — a hard failure, not a warning. Reported once per node, not
            # once per poll.
            if not app_hash:
                if endpoint.name not in empty_hash:
                    empty_hash.add(endpoint.name)
                    divergences.append(
                        {
                            "kind": "diverged",
                            "height": height,
                            "reason": f"{endpoint.name} reported height {height} with "
                            "an empty last_block_app_hash — a committed height always "
                            "has a commit hash, so the field was renamed or dropped "
                            "and nothing could be compared",
                        }
                    )
                continue
            at_height = seen.setdefault(height, {})
            previous = at_height.get(endpoint.name)
            if previous is not None and previous != app_hash:
                divergences.append(
                    {
                        "kind": "diverged",
                        "height": height,
                        "hashes": {endpoint.name: [previous, app_hash]},
                        "reason": f"{endpoint.name} reported two app hashes for "
                        f"height {height}: {previous} then {app_hash}",
                    }
                )
            at_height[endpoint.name] = app_hash
        if start_tip is None and len(tips) == len(endpoints):
            start_tip = min(tips.values())
        if start_tip is not None and min(tips.values()) >= start_tip + blocks:
            break
        if time.monotonic() > deadline:
            break
        time.sleep(interval)

    silent = [endpoint.name for endpoint in endpoints if endpoint.name not in tips]
    if silent:
        divergences.append(
            {
                "kind": "unverified",
                "unreachable": silent,
                "reason": f"no committed app hash from {silent} within {timeout}s — a "
                "node that computes a different app hash panics and stops, so "
                "silence is a divergence symptom, not a pass",
            }
        )
    elif min(tips.values()) < start_tip + blocks:
        divergences.append(
            {
                "kind": "unverified",
                "reason": f"chain only advanced to {min(tips.values())} from "
                f"{start_tip} within {timeout}s, wanted {blocks} more blocks on "
                f"every node: {tips}",
            }
        )

    compared = 0
    for height in sorted(seen):
        hashes = seen[height]
        if len(hashes) < 2:
            continue
        compared += 1
        if len(set(hashes.values())) > 1:
            divergences.append(
                {
                    "kind": "diverged",
                    "height": height,
                    "hashes": hashes,
                    "reason": f"app_hash divergence at height {height}: {hashes}",
                }
            )
    if not compared:
        divergences.append(
            {
                "kind": "unverified",
                "reason": "no height was reported by two nodes — app-hash agreement "
                "was never actually compared",
            }
        )
    return divergences
