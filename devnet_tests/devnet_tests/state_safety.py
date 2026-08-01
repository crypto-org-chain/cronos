import os
import shutil
import subprocess
import time
from dataclasses import dataclass, field
from pathlib import Path
from urllib.parse import urlparse

import requests

from .devnet import Node

# A client-side address that resolves to this machine, i.e. the node process is
# local and its fd table is inspectable.
_LOCAL_HOSTS = {"localhost", "127.0.0.1", "::1", "0.0.0.0"}
# The process that must own the port for its fd table to say anything about the
# node; a docker-proxy or socat forwarder can be listening instead.
_NODE_PROCESS = "cronosd"

# Enough forward samples to see every node commit past a shared starting tip.
APP_HASH_BLOCKS = 3
APP_HASH_INTERVAL_S = 0.5
APP_HASH_TIMEOUT_S = 60


def _http_url(rpc: str) -> str:
    """CometBFT RPC addresses are conventionally configured as `tcp://host:port`,
    a scheme `requests` has no adapter for."""
    if rpc.startswith("tcp://"):
        return "http://" + rpc[len("tcp://") :]
    return rpc


def abci_app_hash(node: Node) -> tuple[int, str]:
    """The (height, app_hash) a node computed for itself, from the app's own last
    commit ID.

    A block header's app_hash is consensus data, byte-identical on every node
    that manages to commit the block at all, so comparing it across nodes can
    never surface a divergence. This is a per-node measurement and can actually
    differ. Returns height 0 while the node has committed nothing.
    """
    rsp = requests.get(f"{_http_url(node.rpc)}/abci_info", timeout=5).json()
    response = rsp["result"]["response"]
    return int(response.get("last_block_height", 0) or 0), response.get(
        "last_block_app_hash", ""
    )


def check_app_hash_agreement(
    nodes: list[Node],
    blocks: int = APP_HASH_BLOCKS,
    interval: float = APP_HASH_INTERVAL_S,
    timeout: float = APP_HASH_TIMEOUT_S,
) -> list[dict]:
    """Sample every node's own computed app hash while `blocks` further blocks
    commit, then report every height where two nodes disagreed.

    Sampling has to run forward in time: a node only exposes the hash it
    computed for its current tip, so past heights cannot be asked for after the
    fact. A node whose execution diverged panics with "wrong Block.Header.AppHash"
    and stops answering, so silence from any node is reported, as is a chain that
    never advances `blocks` heights on every node, a window where no height was
    ever reported by two nodes, and an empty app hash from a node that has
    committed — none of those verified agreement, and an empty result has to mean
    "checked and agreed".
    """
    if len(nodes) < 2:
        return [
            {
                "reason": "need two nodes to compare app hashes, got "
                f"{[node.name for node in nodes]}"
            }
        ]

    divergences: list[dict] = []
    seen: dict[int, dict[str, str]] = {}
    tips: dict[str, int] = {}
    # node -> first height it reported a committed block with no app hash.
    missing_hash: dict[str, int] = {}
    start_tip = None
    deadline = time.monotonic() + timeout
    while True:
        for node in nodes:
            try:
                height, app_hash = abci_app_hash(node)
            except Exception:  # noqa: BLE001
                continue
            if height == 0:
                continue
            # Recorded before the empty-hash check below: the node did answer, so
            # leaving it out of `tips` would also report it as silent and burn the
            # full timeout waiting for a tip it already gave us.
            tips[node.name] = max(tips.get(node.name, 0), height)
            # last_block_app_hash is omitempty, so an absent or renamed key reads
            # as "" on every node and they all compare equal having compared
            # nothing.
            if not app_hash:
                missing_hash.setdefault(node.name, height)
                continue
            at_height = seen.setdefault(height, {})
            previous = at_height.get(node.name)
            if previous is not None and previous != app_hash:
                divergences.append(
                    {
                        "height": height,
                        "hashes": {node.name: [previous, app_hash]},
                        "reason": f"{node.name} reported two app hashes for height "
                        f"{height}: {previous} then {app_hash}",
                    }
                )
            at_height[node.name] = app_hash
        if start_tip is None and len(tips) == len(nodes):
            start_tip = min(tips.values())
        if start_tip is not None and min(tips.values()) >= start_tip + blocks:
            break
        if time.monotonic() > deadline:
            break
        time.sleep(interval)

    silent = [node.name for node in nodes if node.name not in tips]
    if missing_hash:
        divergences.append(
            {
                "height": min(missing_hash.values()),
                "hashes": dict.fromkeys(missing_hash),
                "reason": f"{sorted(missing_hash)} reported a committed height with an "
                "empty app hash — the field is omitempty, so an absent or renamed key "
                "would make every node compare equal without comparing anything",
            }
        )
    if silent:
        divergences.append(
            {
                "height": None,
                "hashes": dict.fromkeys(silent),
                "reason": f"no committed app hash from {silent} within {timeout}s — a "
                "node that computes a different app hash panics and stops, so silence "
                "is a divergence symptom, not a pass",
            }
        )
    elif min(tips.values()) < start_tip + blocks:
        divergences.append(
            {
                "height": None,
                "hashes": dict(tips),
                "reason": f"chain only advanced to {min(tips.values())} from "
                f"{start_tip} within {timeout}s, wanted {blocks} more blocks on "
                "every node",
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
                {"height": height, "hashes": hashes, "reason": "mismatch"}
            )
    if not compared:
        divergences.append(
            {
                "height": None,
                "hashes": {},
                "reason": "no height was reported by two nodes — app-hash agreement "
                "was never actually compared",
            }
        )
    return divergences


def _process_name(pid: int) -> str | None:
    """None when the name isn't readable from here, which callers must treat as
    "identity unverifiable" — never as a match and never as a mismatch."""
    try:
        return Path(f"/proc/{pid}/comm").read_text().strip()
    except OSError:
        pass
    # macOS and other /proc-less hosts. `comm` is the full executable path here,
    # so callers have to match on a substring.
    try:
        name = subprocess.run(
            ["ps", "-p", str(pid), "-o", "comm="],
            capture_output=True,
            text=True,
            timeout=10,
        ).stdout.strip()
    except Exception:  # noqa: BLE001
        return None
    return name or None


def _listening_pid(port: int) -> int | None:
    if not shutil.which("lsof"):
        return None
    try:
        output = subprocess.run(
            ["lsof", "-t", "-sTCP:LISTEN", f"-iTCP:{port}"],
            capture_output=True,
            text=True,
            timeout=10,
        ).stdout.split()
    except Exception:  # noqa: BLE001
        return None
    return int(output[0]) if output else None


def open_fd_count(address: str) -> int | None:
    """Open fd count of the process listening on `address`'s port.

    None when it isn't measurable from here — a remote devnet, no `lsof`, nothing
    listening, or a port held by something other than a verified node process.
    Callers must treat None as "unknown", never as zero.
    """
    parsed = urlparse(address)
    if parsed.hostname not in _LOCAL_HOSTS or parsed.port is None:
        return None
    pid = _listening_pid(parsed.port)
    if pid is None:
        return None
    name = _process_name(pid)
    # A docker-proxy/socat fd table tells us nothing about the node's leak, and an
    # unverifiable name is not evidence that the node holds the port either.
    if name is None or _NODE_PROCESS not in name:
        return None

    fd_dir = f"/proc/{pid}/fd"
    if os.path.isdir(fd_dir):
        try:
            return len(os.listdir(fd_dir))
        except OSError:
            return None
    # macOS and other /proc-less hosts: one lsof row per fd, after the header.
    try:
        proc = subprocess.run(
            ["lsof", "-p", str(pid)], capture_output=True, text=True, timeout=30
        )
    except Exception:  # noqa: BLE001
        return None
    if proc.returncode != 0:
        return None
    rows = proc.stdout.splitlines()
    # A live process always has fds beyond the header row; header-only output
    # means lsof couldn't actually read the table (e.g. permission denied),
    # not that the count is zero.
    if len(rows) <= 1:
        return None
    return len(rows) - 1


@dataclass
class SoakResult:
    # Queries that returned without raising; `completed + len(errors)` is the
    # requested iteration count.
    completed: int = 0
    errors: list[str] = field(default_factory=list)


def historical_query_soak(w3, address: str, height: int, iterations: int) -> SoakResult:
    """Repeatedly query balance at a fixed historical height, the access pattern
    that used to leak one file descriptor per query (memiavl's read-only
    CacheMultiStoreWithVersion path never closed its DB).

    Only reports query errors; a leak is invisible here until the node's fd
    limit is actually exhausted. Pair with `open_fd_count` around the call to
    observe fd growth directly when the node runs on this host.
    """
    result = SoakResult()
    for _ in range(iterations):
        try:
            w3.eth.get_balance(address, height)
            result.completed += 1
        except Exception as exc:  # noqa: BLE001
            result.errors.append(str(exc))
    return result
