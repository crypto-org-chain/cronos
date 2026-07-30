"""Open-loop soak testing.

Paces load at a target tx/s rate for a fixed wall-clock duration (instead of
firing one fixed-size batch and waiting for it to drain), samples periodic
checkpoints of TPS, block time, and RSS, and trend-fits those checkpoints to
flag a memory leak or performance degradation.
"""

import sys
import threading
import time
from statistics import linear_regression

from .stats import _fetch_prometheus, calculate_tps, get_block_info_cosmos
from .resources import scrape_go_runtime
from .utils import block_height

# Sustained growth beyond these slopes over the soak trips the verdict.
LEAK_RSS_SLOPE_BYTES_PER_S = 1024 * 1024  # 1 MiB/s
DEGRADATION_BLOCK_TIME_SLOPE_MS_PER_S = 1.0  # +1ms/s of block time

_TREND_GATES = (
    (
        "rss_bytes",
        LEAK_RSS_SLOPE_BYTES_PER_S,
        "RSS growing {:.0f} bytes/s — possible memory leak",
    ),
    (
        "avg_block_time_ms",
        DEGRADATION_BLOCK_TIME_SLOPE_MS_PER_S,
        "block time growing {:.2f} ms/s — possible degradation",
    ),
)


def _checkpoint(rpc, telemetry, prev_height, cur_height, elapsed_s):
    """One checkpoint's TPS/block-time (over [prev_height, cur_height]) and RSS."""
    blocks = [
        (txs, timestamp)
        for timestamp, txs in (
            get_block_info_cosmos(h, rpc) for h in range(prev_height, cur_height + 1)
        )
    ]
    tps = 0.0
    avg_block_time_ms = None
    if len(blocks) >= 2:
        tps = calculate_tps(blocks)
        span = (blocks[-1][1] - blocks[0][1]).total_seconds()
        avg_block_time_ms = span / (len(blocks) - 1) * 1000

    rss_bytes = None
    if telemetry:
        rss_bytes = scrape_go_runtime(_fetch_prometheus(telemetry))["rss_bytes"]

    return {
        "elapsed_s": elapsed_s,
        "height": cur_height,
        "tps": tps,
        "avg_block_time_ms": avg_block_time_ms,
        "rss_bytes": rss_bytes,
    }


class CheckpointSampler:
    """Background thread sampling soak checkpoints at a fixed interval."""

    def __init__(self, rpc, telemetry, checkpoint_interval):
        self._rpc = rpc
        self._telemetry = telemetry
        self._interval = checkpoint_interval
        self._checkpoints = []
        self._stop = threading.Event()
        self._thread = None
        self._start_time = None
        self._prev_height = None

    def start(self):
        self._start_time = time.monotonic()
        self._prev_height = block_height(self._rpc)
        self._thread = threading.Thread(target=self._poll, daemon=True)
        self._thread.start()

    def stop(self):
        self._stop.set()
        if self._thread:
            self._thread.join(timeout=self._interval + 5)
            if self._thread.is_alive():
                print("warning: checkpoint sampler thread did not stop in time", file=sys.stderr)

    @property
    def checkpoints(self):
        return list(self._checkpoints)

    def _poll(self):
        while not self._stop.wait(self._interval):
            try:
                cur_height = block_height(self._rpc)
                elapsed = time.monotonic() - self._start_time
                self._checkpoints.append(
                    _checkpoint(self._rpc, self._telemetry, self._prev_height, cur_height, elapsed)
                )
                # Next window starts at cur_height, not cur_height + 1: cur_height
                # is both this window's last counted block and next window's
                # anchor, so no block's tx count falls in the gap between windows.
                self._prev_height = cur_height
            except Exception:
                pass


def fit_trends(checkpoints):
    """{metric: slope-per-second} via least squares over elapsed_s, skipping
    metrics with fewer than two non-None samples."""
    trends = {}
    for key in ("tps", "avg_block_time_ms", "rss_bytes"):
        points = [(c["elapsed_s"], c[key]) for c in checkpoints if c.get(key) is not None]
        if len(points) < 2:
            trends[key] = None
            continue
        xs, ys = zip(*points)
        slope, _ = linear_regression(xs, ys)
        trends[key] = slope
    return trends


def soak_verdict(trends):
    """Flag sustained RSS growth (leak) or block-time growth (degradation)."""
    reasons = [
        message.format(trends[key])
        for key, max_slope, message in _TREND_GATES
        if trends.get(key) is not None and trends[key] > max_slope
    ]
    return {"ok": not reasons, "reasons": reasons}
