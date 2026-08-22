import sys
import threading

from .stats import _fetch_prometheus, scrape_blockstm_metrics
from .utils import block_height, mempool_status, txpool_status


class _HeightSampler:
    """Daemon thread that samples a metric per block height until stopped.

    Subclasses implement `_poll`, which must loop on `self._stop.is_set()`,
    swallow transport errors, and pace itself with `self._stop.wait(...)` so
    `stop()` interrupts the sleep instead of waiting it out.
    """

    def __init__(self, rpc, interval):
        self._rpc = rpc
        self._interval = interval
        self._data = {}
        self._lock = threading.Lock()
        self._stop = threading.Event()
        self._thread = None

    def start(self):
        self._thread = threading.Thread(target=self._poll, daemon=True)
        self._thread.start()

    def stop(self):
        self._stop.set()
        if self._thread:
            self._thread.join(timeout=2)
            if self._thread.is_alive():
                print(
                    f"warning: {type(self).__name__} poll thread did not stop within timeout; "
                    "collected data may still be mutating",
                    file=sys.stderr,
                )

    @property
    def data(self):
        """Snapshot keyed by block height; the value shape is per subclass."""
        with self._lock:
            return dict(self._data)

    def _poll(self):
        raise NotImplementedError


class MempoolMonitor(_HeightSampler):
    """Background thread that polls mempool pressure during the load period.

    Records the peak (n_txs, n_bytes) observed at each block height so that
    dump_block_stats can report accurate mempool pressure instead of always
    seeing 0 when queried post-hoc.

    `mempool.type=app` bypasses CometBFT's own mempool, so its
    `/num_unconfirmed_txs` always reads 0 in that mode - poll the eth
    `txpool_status` RPC (backed by the app-mempool client) instead by passing
    `json_rpc`. `n_bytes` is always 0 in that mode; app-mempool exposes no
    byte-size equivalent.

    `txpool_status` is expensive (it walks the app-mempool client), so it's
    only fetched once per new block height rather than on every `interval`
    tick - each block's peak is a single sample.
    """

    def __init__(self, rpc, json_rpc=None, interval=0.2):
        super().__init__(rpc, interval)
        self._json_rpc = json_rpc

    def _poll(self):
        last_txpool_height = None
        n_txs, n_bytes = 0, 0
        while not self._stop.is_set():
            try:
                h = block_height(self._rpc)
                if self._json_rpc:
                    if h != last_txpool_height:
                        n_txs, n_bytes = txpool_status(self._json_rpc)
                        last_txpool_height = h
                else:
                    n_txs, n_bytes = mempool_status(self._rpc)
                prev = self._data.get(h, (0, 0))
                with self._lock:
                    self._data[h] = (max(prev[0], n_txs), max(prev[1], n_bytes))
            except Exception:
                pass
            self._stop.wait(self._interval)


class BlockSTMMonitor(_HeightSampler):
    """Background thread that records Block-STM gauges at each new block height.

    The Cosmos SDK Block-STM executor sets Prometheus gauges (not counters)
    that are overwritten on every FinalizeBlock. This monitor polls the
    telemetry endpoint and captures the (executed_txs, validated_txs) snapshot
    whenever the block height advances, so dump_block_stats can report
    accurate per-block averages instead of a stale post-hoc value.
    """

    def __init__(self, rpc, telemetry, interval=0.3):
        super().__init__(rpc, interval)
        self._telemetry = telemetry
        self._last_height = 0

    def _poll(self):
        while not self._stop.is_set():
            try:
                h = block_height(self._rpc)
                if h != self._last_height:
                    self._last_height = h
                    prom_text = _fetch_prometheus(self._telemetry)
                    stm = scrape_blockstm_metrics(prom_text)
                    if stm:
                        executed = stm.get("executed_txs", 0)
                        validated = stm.get("validated_txs", 0)
                        with self._lock:
                            self._data[h] = (executed, validated)
            except Exception:
                pass
            self._stop.wait(self._interval)
