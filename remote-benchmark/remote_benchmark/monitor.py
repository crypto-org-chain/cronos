import threading

from .stats import _fetch_prometheus, scrape_blockstm_metrics
from .utils import block_height, mempool_status


class MempoolMonitor:
    """Background thread that polls CometBFT mempool during the load period.

    Records the peak (n_txs, n_bytes) observed at each block height so that
    dump_block_stats can report accurate mempool pressure instead of always
    seeing 0 when queried post-hoc.
    """

    def __init__(self, rpc, interval=0.2):
        self._rpc = rpc
        self._interval = interval
        self._data = {}
        self._stop = threading.Event()
        self._thread = None

    def start(self):
        self._thread = threading.Thread(target=self._poll, daemon=True)
        self._thread.start()

    def stop(self):
        self._stop.set()
        if self._thread:
            self._thread.join(timeout=2)

    @property
    def data(self):
        """Dict mapping block height to (peak_n_txs, peak_n_bytes)."""
        return dict(self._data)

    def _poll(self):
        while not self._stop.is_set():
            try:
                h = block_height(self._rpc)
                n_txs, n_bytes = mempool_status(self._rpc)
                prev = self._data.get(h, (0, 0))
                self._data[h] = (max(prev[0], n_txs), max(prev[1], n_bytes))
            except Exception:
                pass
            self._stop.wait(self._interval)


class BlockSTMMonitor:
    """Background thread that records Block-STM gauges at each new block height.

    The Cosmos SDK Block-STM executor sets Prometheus gauges (not counters)
    that are overwritten on every FinalizeBlock. This monitor polls the
    telemetry endpoint and captures the (executed_txs, validated_txs) snapshot
    whenever the block height advances, so dump_block_stats can report
    accurate per-block averages instead of a stale post-hoc value.
    """

    def __init__(self, rpc, telemetry, interval=0.3):
        self._rpc = rpc
        self._telemetry = telemetry
        self._interval = interval
        self._data = {}  # height -> (executed, validated)
        self._stop = threading.Event()
        self._thread = None
        self._last_height = 0

    def start(self):
        self._thread = threading.Thread(target=self._poll, daemon=True)
        self._thread.start()

    def stop(self):
        self._stop.set()
        if self._thread:
            self._thread.join(timeout=2)

    @property
    def data(self):
        """Dict mapping block height to (executed_txs, validated_txs)."""
        return dict(self._data)

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
                        if executed > 0:
                            self._data[h] = (executed, validated)
            except Exception:
                pass
            self._stop.wait(self._interval)
