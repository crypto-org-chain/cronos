import threading
import time

import requests


class LoadGenerator:
    """Repeatedly calls send_fn() on a background thread and records each
    attempt's outcome in order, so a test can assert on the tail of the list
    (sends made once some later step completed) while tolerating failures
    during a brief expected outage in the middle (e.g. a node restart)."""

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
        self._thread.join()


def app_hash_at(rpc_port, height):
    url = f"http://127.0.0.1:{rpc_port}/block?height={height}"
    rsp = requests.get(url, timeout=5).json()
    return rsp["result"]["block"]["header"]["app_hash"]


def assert_no_divergence(rpc_ports, start, end):
    """Raise if any two nodes disagree on the app_hash at any height in
    [start, end]. Heights not yet produced when the fastest node was queried
    are excluded, not treated as a mismatch."""
    for height in range(start, end + 1):
        hashes = {}
        for port in rpc_ports:
            try:
                hashes[port] = app_hash_at(port, height)
            except Exception:  # noqa: BLE001
                continue
        if len(set(hashes.values())) > 1:
            raise AssertionError(f"app_hash divergence at height {height}: {hashes}")
