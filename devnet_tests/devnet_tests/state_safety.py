from dataclasses import dataclass, field

import requests

from .devnet import Node


def app_hash_at(node: Node, height: int) -> str:
    rsp = requests.get(f"{node.rpc}/block?height={height}", timeout=5).json()
    return rsp["result"]["block"]["header"]["app_hash"]


def check_app_hash_agreement(nodes: list[Node], start: int, end: int) -> list[dict]:
    """Compare app_hash across every node that reached each height in [start, end].

    A node that errored on a height is left out of that height's `hashes`
    instead of counting as a mismatch, so only nodes that actually replied with
    differing hashes are flagged. A height fewer than 2 nodes answered was never
    verified (e.g. a total outage), so it's reported as unverifiable rather than
    read as agreement.
    """
    divergences = []
    for height in range(start, end + 1):
        hashes = {}
        for node in nodes:
            try:
                hashes[node.name] = app_hash_at(node, height)
            except Exception:
                continue
        if len(hashes) < 2:
            reason = "unverifiable"
        elif len(set(hashes.values())) > 1:
            reason = "mismatch"
        else:
            continue
        divergences.append({"height": height, "hashes": hashes, "reason": reason})
    return divergences


@dataclass
class SoakResult:
    iterations: int
    errors: list[str] = field(default_factory=list)


def historical_query_soak(w3, address: str, height: int, iterations: int) -> SoakResult:
    """Repeatedly query balance at a fixed historical height, the access pattern
    that used to leak one file descriptor per query (memiavl's read-only
    CacheMultiStoreWithVersion path never closed its DB). No RPC exposes an fd
    count, so errors once the node's ulimit is exhausted are the only observable
    signal of a regression."""
    errors = []
    for _ in range(iterations):
        try:
            w3.eth.get_balance(address, height)
        except Exception as exc:  # noqa: BLE001
            errors.append(str(exc))
    return SoakResult(iterations=iterations, errors=errors)
