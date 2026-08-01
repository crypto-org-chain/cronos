"""Open-loop soak testing.

Paces load at a target tx/s rate for a fixed wall-clock duration (instead of
firing one fixed-size batch and waiting for it to drain), samples periodic
checkpoints of TPS, block time, and RSS, and trend-fits those checkpoints to
flag a memory leak or performance degradation.
"""

import logging
import sys
import threading
import time
from statistics import linear_regression, median

from .stats import _fetch_prometheus, calculate_tps, get_block_info_cosmos
from .resources import scrape_go_runtime
from .utils import block_height

log = logging.getLogger(__name__)

# Trend gates compare the fitted trend's total change over the soak against an
# opening baseline. Relative, not per-second: an absolute slope gate gets
# weaker the longer the soak runs — a 500 -> 0 tx/s collapse spread over 600s is
# only -0.83 tx/s per second and would slip under any useful absolute threshold.
LEAK_RSS_GROWTH_FRAC = 0.25
DEGRADATION_BLOCK_TIME_GROWTH_FRAC = 0.5
DEGRADATION_TPS_DROP_FRAC = 0.5

# How many opening checkpoints the baseline is taken over (as a median), capped
# at half the run so a short soak still has a baseline distinct from its tail.
BASELINE_CHECKPOINTS = 3

# Below this, no checkpoint saw the chain commit real load: a run where every tx
# is rejected trends as flat, indistinguishable from healthy steady throughput.
MIN_SOAK_TPS = 0.5

# (metric, sign, max_change_frac, message). sign is +1 when growth is the failure
# and -1 when decay is, so `sign * change_frac > max_change_frac` gates both
# directions. Messages take (change percent, slope per second).
_TREND_GATES = (
    (
        "rss_bytes",
        1,
        LEAK_RSS_GROWTH_FRAC,
        "RSS grew {:.0f}% over the soak ({:.0f} bytes/s) — possible memory leak",
    ),
    (
        "avg_block_time_ms",
        1,
        DEGRADATION_BLOCK_TIME_GROWTH_FRAC,
        "block time grew {:.0f}% over the soak ({:.2f} ms/s) — possible degradation",
    ),
    (
        "tps",
        -1,
        DEGRADATION_TPS_DROP_FRAC,
        "throughput fell {:.0f}% over the soak ({:.2f} tx/s per second) — "
        "possible degradation",
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
    # An interval with no new block has no measurable throughput; recording 0.0
    # would feed the decay-trend gate a sample the chain never produced.
    tps = None
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
                log.debug("checkpoint sample failed", exc_info=True)


def _metric_points(checkpoints, key):
    """[(elapsed_s, value)] for one metric, dropping unsampled checkpoints."""
    return [(c["elapsed_s"], c[key]) for c in checkpoints if c.get(key) is not None]


def _split_baseline(points):
    """(warm-up head, steady tail) for one metric's samples.

    A node's Go heap and memiavl caches are still filling at the opening
    checkpoints, so the ramp belongs in the baseline and out of the trend: left
    in the fit it tilts the slope and reads warm-up as growth.

    The tail keeps at least two samples so a trend can still be fitted; when the
    series is too short to give both a head and a fittable tail, the tail falls
    back to the whole series and the ramp is simply included — a short soak that
    evaluated its gates on a slightly tilted slope beats one that evaluated
    nothing.
    """
    head_size = max(1, min(BASELINE_CHECKPOINTS, len(points) // 2))
    tail = points[head_size:]
    return points[:head_size], tail if len(tail) >= 2 else points


def fit_trends(checkpoints):
    """{metric: slope-per-second} via least squares over elapsed_s, fitted over
    the steady window only (see _split_baseline) and skipping metrics with fewer
    than two non-None samples."""
    trends = {}
    for key in ("tps", "avg_block_time_ms", "rss_bytes"):
        points = _metric_points(checkpoints, key)
        if len(points) < 2:
            trends[key] = None
            continue
        _, steady = _split_baseline(points)
        xs, ys = zip(*steady)
        slope, _ = linear_regression(xs, ys)
        trends[key] = slope
    return trends


def trend_change_fraction(checkpoints, key, slope):
    """The fitted trend's total change over the steady window, as a fraction of
    the opening baseline.

    Both sides skip the warm-up window: the baseline is the median of the opening
    samples rather than the single cold one, and the span the slope is projected
    over is the same steady window the slope was fitted on.

    None when there is no span, no slope, or no positive baseline to divide by
    (a metric that starts at zero has no meaningful relative change — the TPS
    floor gate covers that case instead).
    """
    if slope is None:
        return None
    points = _metric_points(checkpoints, key)
    if len(points) < 2:
        return None
    head, steady = _split_baseline(points)
    span = steady[-1][0] - steady[0][0]
    baseline = median(value for _, value in head)
    if span <= 0 or baseline <= 0:
        return None
    return slope * span / baseline


def soak_verdict(trends, checkpoints, telemetry=None):
    """Flag sustained RSS growth (leak), block-time/throughput degradation, or a
    run that never produced throughput at all.

    Trend gates measure each metric's fitted change over the whole soak relative
    to its opening baseline, so a slow-motion collapse over a long soak trips the
    same gate as a fast one. They need at least two checkpoints; below that no
    gate can evaluate, so the soak verified nothing and is reported as not ok
    rather than vacuously passing. Enough checkpoints is necessary but not
    sufficient:

    - A halted chain or missing telemetry leaves every metric None, so gates are
      counted as they evaluate.
    - A run where every tx is rejected produces empty blocks, and a flat zero
      TPS fits the same zero slope as healthy steady throughput — hence the
      separate floor gate on the checkpoint TPS values themselves.
    - The leak gate is the soak's stated purpose, so when telemetry was
      configured but never yielded RSS it fails loud instead of letting the
      cheaper block-derived gates carry the verdict.

    Returns {ok, reasons, gates}, where `gates` maps each gate to "passed",
    "failed", or "not evaluated" so a caller can tell a gate that ran and
    passed from one that never ran.
    """
    num_checkpoints = len(checkpoints)
    if num_checkpoints < 2:
        return {
            "ok": False,
            "reasons": [
                f"only {num_checkpoints} checkpoint(s) recorded — need 2 to fit a "
                "trend, soak verified nothing"
            ],
            "gates": {},
        }

    reasons = []
    gates = {}
    for key, sign, max_change_frac, message in _TREND_GATES:
        slope = trends.get(key)
        change_frac = trend_change_fraction(checkpoints, key, slope)
        if change_frac is None:
            gates[f"{key}_trend"] = "not evaluated"
            continue
        if sign * change_frac > max_change_frac:
            # Formatted as magnitudes: the gate's sign convention already
            # encodes the failing direction, and a "-" after "fell" reads
            # backwards.
            reasons.append(message.format(abs(change_frac) * 100, abs(slope)))
            gates[f"{key}_trend"] = "failed"
        else:
            gates[f"{key}_trend"] = "passed"

    tps_values = [c["tps"] for c in checkpoints if c.get("tps") is not None]
    if not tps_values:
        gates["tps_floor"] = "not evaluated"
    elif max(tps_values) < MIN_SOAK_TPS:
        gates["tps_floor"] = "failed"
        reasons.append(
            f"peak checkpoint throughput {max(tps_values):.2f} tx/s < "
            f"{MIN_SOAK_TPS:.2f} tx/s — chain produced no throughput, so a flat "
            "TPS trend means dead, not stable"
        )
    else:
        gates["tps_floor"] = "passed"

    if telemetry and gates["rss_bytes_trend"] == "not evaluated":
        reasons.append(
            f"no RSS samples from telemetry {telemetry} — the leak gate, the "
            "soak's purpose, never ran"
        )

    if all(state == "not evaluated" for state in gates.values()):
        return {
            "ok": False,
            "reasons": [
                "no gates had data - soak unmeasured (no telemetry and no "
                "block progress across checkpoints)"
            ],
            "gates": gates,
        }

    return {"ok": not reasons, "reasons": reasons, "gates": gates}
