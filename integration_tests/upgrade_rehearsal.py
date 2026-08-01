import threading
import time

import requests


class LoadGenerator:
    """Repeatedly calls send_fn() on a background thread and records each
    attempt's outcome in order, so a test can assert on the tail of the list
    (sends made once some later step completed) while tolerating failures
    during a brief expected outage in the middle (e.g. a node restart).

    send_fn must bound its own wait (web3's receipt wait defaults to 120s),
    otherwise stop() cannot join the worker within its timeout."""

    def __init__(self, send_fn, interval=0.5):
        self._send_fn = send_fn
        self._interval = interval
        self._stop = threading.Event()
        self._thread = threading.Thread(target=self._run, daemon=True)
        self.results = []

    def _run(self):
        while not self._stop.is_set():
            try:
                self._send_fn()
                self.results.append(True)
            except Exception:  # noqa: BLE001
                self.results.append(False)
            time.sleep(self._interval)

    def start(self):
        self._thread.start()
        return self

    def stop(self):
        self._stop.set()
        self._thread.join(timeout=self._interval + 10)
        assert not self._thread.is_alive(), "LoadGenerator thread did not stop in time"


def abci_app_hash(rpc_port):
    """The (height, app_hash) a node computed for itself, read from the app's
    own last commit ID via /abci_info. Unlike a block header's app_hash - which
    consensus makes byte-identical on every node that manages to commit the
    block at all - this is a per-node measurement and can actually differ."""
    url = f"http://127.0.0.1:{rpc_port}/abci_info"
    rsp = requests.get(url, timeout=5).json()["result"]["response"]
    # both fields are omitted while nothing has been committed yet
    return int(rsp.get("last_block_height", 0)), rsp.get("last_block_app_hash", "")


def assert_no_divergence(rpc_ports, blocks=5, interval=0.2, timeout=120):
    """Sample every node's own computed app hash while `blocks` further blocks
    commit, then raise if two nodes ever reported different hashes for the same
    height.

    Sampling has to run forward in time: a node only ever exposes the hash it
    computed for its current tip, so past heights cannot be asked for after the
    fact. A node whose execution diverged panics with "wrong Block.Header.AppHash"
    and stops answering, so silence from any node is treated as a failure too,
    as is a chain that never advances `blocks` heights."""
    if blocks < 1:
        raise AssertionError(f"blocks must be at least 1, got {blocks}")
    if len(rpc_ports) < 2:
        raise AssertionError(f"need two nodes to compare, got {rpc_ports}")

    seen = {}  # height -> {port: app_hash}
    tips = {}  # port -> highest height it reported
    start_tip = None
    deadline = time.monotonic() + timeout
    while True:
        for port in rpc_ports:
            try:
                height, app_hash = abci_app_hash(port)
            except Exception:  # noqa: BLE001
                continue
            if height == 0:
                continue
            # last_block_app_hash is omitempty in the abci response, so a
            # renamed/absent field reads back as "" on every node and would
            # "match" without ever comparing a hash. A committed height always
            # has a non-empty hash, so treat emptiness as a failure.
            if not app_hash:
                raise AssertionError(
                    f"node on port {port} reported height {height} with an empty "
                    "app hash: last_block_app_hash is missing from /abci_info, "
                    "so no hash was actually compared"
                )
            tips[port] = max(tips.get(port, 0), height)
            at_height = seen.setdefault(height, {})
            previous = at_height.get(port)
            if previous is not None and previous != app_hash:
                raise AssertionError(
                    f"node on port {port} reported two app hashes for height "
                    f"{height}: {previous} then {app_hash}"
                )
            at_height[port] = app_hash
        if start_tip is None and len(tips) == len(rpc_ports):
            start_tip = min(tips.values())
        if start_tip is not None and min(tips.values()) >= start_tip + blocks:
            break
        if time.monotonic() > deadline:
            break
        time.sleep(interval)

    silent = [port for port in rpc_ports if port not in tips]
    if silent:
        raise AssertionError(
            f"no committed app hash from {silent} within {timeout}s - a node that "
            "computes a different app hash panics and stops, so silence is a "
            "divergence symptom, not a pass"
        )
    if min(tips.values()) < start_tip + blocks:
        raise AssertionError(
            f"chain only advanced to {min(tips.values())} from {start_tip} within "
            f"{timeout}s, wanted {blocks} more blocks on every node: {tips}"
        )

    compared = 0
    for height in sorted(seen):
        hashes = seen[height]
        if len(hashes) < 2:
            continue
        compared += 1
        if len(set(hashes.values())) > 1:
            raise AssertionError(f"app_hash divergence at height {height}: {hashes}")
    if compared == 0:
        raise AssertionError(
            f"no height was reported by two nodes among {rpc_ports}: nothing "
            f"compared (tips {tips})"
        )
