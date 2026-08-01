import threading
import time
from datetime import datetime, timedelta, timezone

import pytest

from remote_benchmark import soak as soak_module
from remote_benchmark.soak import (
    CheckpointSampler,
    _checkpoint,
    fit_trends,
    soak_verdict,
    trend_change_fraction,
)

MIB = 1024 * 1024
# The absolute per-second thresholds these gates used before they were made
# relative; the realistic-magnitude regressions below all slip under them.
OLD_RSS_SLOPE_BYTES_PER_S = MIB
OLD_BLOCK_TIME_SLOPE_MS_PER_S = 1.0
OLD_TPS_SLOPE_PER_S = 1.0


def _dt(seconds):
    return datetime(2026, 1, 1, tzinfo=timezone.utc) + timedelta(seconds=seconds)


def test_checkpoint_computes_tps_and_block_time_from_block_range(monkeypatch):
    blocks = {
        10: (_dt(0), 5),
        11: (_dt(2), 5),
        12: (_dt(4), 10),
    }
    monkeypatch.setattr(soak_module, "get_block_info_cosmos", lambda h, rpc: blocks[h])
    monkeypatch.setattr(soak_module, "scrape_go_runtime", lambda text: {"rss_bytes": 999.0})
    monkeypatch.setattr(soak_module, "_fetch_prometheus", lambda url: "text" if url else "")

    cp = _checkpoint("http://rpc", "http://telemetry", 10, 12, elapsed_s=4.0)

    assert cp["height"] == 12
    assert cp["avg_block_time_ms"] == 2000.0
    assert cp["tps"] == 15 / 4  # blocks 11,12 txs (anchor block 10 excluded) over 4s span
    assert cp["rss_bytes"] == 999.0


def test_checkpoint_without_telemetry_has_no_rss(monkeypatch):
    blocks = {10: (_dt(0), 0), 11: (_dt(1), 0)}
    monkeypatch.setattr(soak_module, "get_block_info_cosmos", lambda h, rpc: blocks[h])

    cp = _checkpoint("http://rpc", None, 10, 11, elapsed_s=1.0)

    assert cp["rss_bytes"] is None


def test_checkpoint_has_no_tps_when_no_block_was_produced(monkeypatch):
    # A fake 0.0 here would reach the decay-trend gate as a real sample.
    blocks = {10: (_dt(0), 5)}
    monkeypatch.setattr(soak_module, "get_block_info_cosmos", lambda h, rpc: blocks[h])

    cp = _checkpoint("http://rpc", None, 10, 10, elapsed_s=1.0)

    assert cp["tps"] is None
    assert cp["avg_block_time_ms"] is None


def test_fit_trends_ignores_checkpoints_without_tps():
    checkpoints = [
        {"elapsed_s": 0, "tps": 10.0, "avg_block_time_ms": None, "rss_bytes": None},
        {"elapsed_s": 10, "tps": None, "avg_block_time_ms": None, "rss_bytes": None},
        {"elapsed_s": 20, "tps": 10.0, "avg_block_time_ms": None, "rss_bytes": None},
    ]

    assert fit_trends(checkpoints)["tps"] == 0.0


def test_fit_trends_detects_growing_rss():
    checkpoints = [
        {"elapsed_s": 0, "tps": 10.0, "avg_block_time_ms": 500.0, "rss_bytes": 100_000_000},
        {"elapsed_s": 10, "tps": 10.0, "avg_block_time_ms": 500.0, "rss_bytes": 100_000_000 + 20 * MIB},
        {"elapsed_s": 20, "tps": 10.0, "avg_block_time_ms": 500.0, "rss_bytes": 100_000_000 + 40 * MIB},
    ]

    trends = fit_trends(checkpoints)

    assert trends["rss_bytes"] > OLD_RSS_SLOPE_BYTES_PER_S
    assert trends["avg_block_time_ms"] == 0.0
    assert trends["tps"] == 0.0


def test_fit_trends_returns_none_for_metric_with_insufficient_samples():
    checkpoints = [{"elapsed_s": 0, "tps": 1.0, "avg_block_time_ms": None, "rss_bytes": None}]

    trends = fit_trends(checkpoints)

    assert trends["avg_block_time_ms"] is None
    assert trends["rss_bytes"] is None


def _checkpoints(tps=100.0, n=3, block_time_ms=500.0, rss_bytes=500 * MIB, interval=30):
    """n checkpoints `interval` seconds apart. A (start, end) tuple for a metric
    ramps it linearly across the run; a scalar holds it flat."""

    def value(metric, i):
        if isinstance(metric, tuple):
            start, end = metric
            return start + (end - start) * i / (n - 1)
        return metric

    return [
        {
            "elapsed_s": interval * i,
            "height": 100 + i,
            "tps": value(tps, i),
            "avg_block_time_ms": value(block_time_ms, i),
            "rss_bytes": value(rss_bytes, i),
        }
        for i in range(n)
    ]


def test_soak_verdict_flags_rss_growth_an_absolute_slope_gate_would_have_missed():
    # 500 MiB of growth over a 600s soak is only ~874 KB/s — under the old
    # 1 MiB/s gate — but half the baseline footprint.
    checkpoints = _checkpoints(n=21, rss_bytes=(1000 * MIB, 1500 * MIB))
    trends = fit_trends(checkpoints)

    assert trends["rss_bytes"] < OLD_RSS_SLOPE_BYTES_PER_S

    verdict = soak_verdict(trends, checkpoints)

    assert verdict["ok"] is False
    assert verdict["gates"]["rss_bytes_trend"] == "failed"
    # 41%, not 50%: the baseline is the median of the opening checkpoints and the
    # slope is projected over the steady window, both of which skip the warm-up.
    assert "RSS grew 41%" in verdict["reasons"][0]
    assert "memory leak" in verdict["reasons"][0]


def test_soak_verdict_flags_block_time_doubling_over_a_long_soak():
    # 500ms -> 1000ms over 600s is +0.83 ms/s, under the old absolute gate.
    checkpoints = _checkpoints(n=21, block_time_ms=(500.0, 1000.0))
    trends = fit_trends(checkpoints)

    assert trends["avg_block_time_ms"] < OLD_BLOCK_TIME_SLOPE_MS_PER_S

    verdict = soak_verdict(trends, checkpoints)

    assert verdict["ok"] is False
    assert verdict["gates"]["avg_block_time_ms_trend"] == "failed"
    assert "block time grew 81%" in verdict["reasons"][0]


def test_soak_verdict_flags_throughput_collapse_over_a_long_soak():
    # 500 -> 0 tx/s over 600s is -0.83 tx/s per second, under the old absolute
    # gate, and the TPS floor gate can't catch it either: the peak was a
    # healthy 500 tx/s.
    checkpoints = _checkpoints(n=21, tps=(500.0, 0.0))
    trends = fit_trends(checkpoints)

    assert abs(trends["tps"]) < OLD_TPS_SLOPE_PER_S

    verdict = soak_verdict(trends, checkpoints)

    assert verdict["ok"] is False
    assert verdict["gates"]["tps_trend"] == "failed"
    assert verdict["gates"]["tps_floor"] == "passed"
    assert "throughput fell 89%" in verdict["reasons"][0]


def test_soak_verdict_does_not_read_cold_start_ramp_up_as_a_leak():
    # RSS climbs from 300 to 500 MiB over the first two checkpoints as the Go
    # heap and memiavl caches warm up, then holds flat. Baselining strictly on
    # the first checkpoint reads that warmup as 29% growth and fails a healthy
    # soak.
    checkpoints = _checkpoints(n=13, rss_bytes=500 * MIB)
    checkpoints[0]["rss_bytes"] = 300 * MIB
    checkpoints[1]["rss_bytes"] = 480 * MIB
    trends = fit_trends(checkpoints)

    # The warm-up window is excluded from the fit too, so it can't tilt the
    # trend up either.
    assert trends["rss_bytes"] == 0.0

    verdict = soak_verdict(trends, checkpoints)

    assert verdict["ok"] is True
    assert verdict["gates"]["rss_bytes_trend"] == "passed"


def test_trend_change_fraction_baselines_on_the_median_of_the_opening_samples():
    checkpoints = _checkpoints(n=6, rss_bytes=(100.0, 200.0))
    checkpoints[0]["rss_bytes"] = 10.0
    trends = fit_trends(checkpoints)

    change = trend_change_fraction(checkpoints, "rss_bytes", trends["rss_bytes"])

    # The slope comes from the steady window (160 -> 200 over 60s), the baseline
    # is median(10, 120, 140) = 120 rather than the 10 of the cold first sample,
    # and the span is that same 60s window, not the full 150s.
    assert trends["rss_bytes"] == pytest.approx(40 / 60)
    assert change == pytest.approx(40 / 60 * 60 / 120)


def test_soak_verdict_ok_for_a_long_stable_soak():
    checkpoints = _checkpoints(n=21)
    verdict = soak_verdict(fit_trends(checkpoints), checkpoints)

    assert verdict["ok"] is True
    assert verdict["reasons"] == []
    assert set(verdict["gates"].values()) == {"passed"}


def test_soak_verdict_ok_when_stable():
    checkpoints = _checkpoints()
    verdict = soak_verdict(fit_trends(checkpoints), checkpoints)

    assert verdict["ok"] is True
    assert verdict["reasons"] == []
    assert set(verdict["gates"].values()) == {"passed"}


def test_soak_verdict_flags_a_run_that_never_produced_throughput():
    # Every tx rejected at CheckTx: blocks are empty, so tps is a flat 0.0 and
    # fits the same zero slope as healthy steady throughput.
    checkpoints = _checkpoints(tps=0.0)
    trends = fit_trends(checkpoints)

    assert trends["tps"] == 0.0  # slope alone looks stable

    verdict = soak_verdict(trends, checkpoints)

    assert verdict["ok"] is False
    assert "chain produced no throughput" in verdict["reasons"][0]
    assert verdict["gates"]["tps_floor"] == "failed"
    # A zero baseline has no meaningful relative change; the floor gate is what
    # catches this run.
    assert verdict["gates"]["tps_trend"] == "not evaluated"


def test_soak_verdict_not_ok_without_any_checkpoint():
    verdict = soak_verdict(fit_trends([]), [])

    assert verdict["ok"] is False
    assert "soak verified nothing" in verdict["reasons"][0]


def test_soak_verdict_not_ok_with_a_single_checkpoint():
    # One checkpoint can't produce a trend, so every gate would be skipped and
    # the verdict would vacuously pass.
    checkpoints = [
        {"elapsed_s": 0, "tps": 10.0, "avg_block_time_ms": 500.0, "rss_bytes": 1_000}
    ]
    trends = fit_trends(checkpoints)

    assert all(slope is None for slope in trends.values())

    verdict = soak_verdict(trends, checkpoints)

    assert verdict["ok"] is False
    assert "soak verified nothing" in verdict["reasons"][0]


def test_soak_verdict_not_ok_when_no_gate_had_data():
    # Enough checkpoints to fit a trend, but a halted chain with no telemetry
    # leaves every metric None, so no gate can evaluate and the verdict must
    # not vacuously pass.
    checkpoints = _checkpoints(tps=None, block_time_ms=None, rss_bytes=None)
    trends = fit_trends(checkpoints)

    verdict = soak_verdict(trends, checkpoints)

    assert verdict["ok"] is False
    assert "no gates had data" in verdict["reasons"][0]


def test_soak_verdict_ok_when_only_some_gates_had_data():
    # A soak without telemetry still has real block-derived gates; those alone
    # are enough for a pass.
    checkpoints = _checkpoints(rss_bytes=None)
    verdict = soak_verdict(fit_trends(checkpoints), checkpoints)

    assert verdict["ok"] is True
    assert verdict["gates"]["rss_bytes_trend"] == "not evaluated"


def test_soak_verdict_fails_when_configured_telemetry_yielded_no_rss():
    # The leak gate is the soak's purpose; the cheap tps gate must not carry the
    # verdict when the scrape silently never produced a sample.
    checkpoints = _checkpoints(rss_bytes=None)
    verdict = soak_verdict(
        fit_trends(checkpoints),
        checkpoints,
        telemetry="http://telemetry/metrics",
    )

    assert verdict["ok"] is False
    assert "the leak gate, the soak's purpose, never ran" in verdict["reasons"][0]
    assert verdict["gates"]["rss_bytes_trend"] == "not evaluated"


def test_checkpoint_sampler_chains_windows_without_gap_or_double_count(monkeypatch):
    blocks = {10: (_dt(0), 0), 11: (_dt(1), 5), 12: (_dt(2), 5), 13: (_dt(3), 5)}
    monkeypatch.setattr(soak_module, "get_block_info_cosmos", lambda h, rpc: blocks[h])
    # The real _poll drives the chaining, so an off-by-one in _prev_height has
    # to show up here rather than in a reimplementation of the loop body.
    tips = iter([12, 13])
    monkeypatch.setattr(soak_module, "block_height", lambda rpc: next(tips))

    sampler = CheckpointSampler("http://rpc", None, checkpoint_interval=0)
    sampler._start_time = time.monotonic()
    sampler._prev_height = 10
    # Two polls, then the third wait() returns True and ends the loop.
    stops = iter([False, False, True])
    monkeypatch.setattr(sampler._stop, "wait", lambda _timeout: next(stops))

    sampler._poll()

    checkpoint_one, checkpoint_two = sampler.checkpoints
    # Window one covers blocks 11,12 (anchor 10 excluded); window two covers
    # just block 13 (anchor 12 excluded) — block 12's txs counted once, not
    # skipped and not double-counted.
    assert (checkpoint_one["height"], checkpoint_one["tps"]) == (12, 10 / 2)
    assert (checkpoint_two["height"], checkpoint_two["tps"]) == (13, 5 / 1)
    assert sampler._prev_height == 13


def test_checkpoint_sampler_stop_warns_if_thread_does_not_exit(monkeypatch, capsys):
    sampler = CheckpointSampler("http://rpc", None, checkpoint_interval=0.01)
    sampler._thread = threading.Thread(target=lambda: time.sleep(10), daemon=True)
    sampler._thread.start()

    sampler.stop()

    assert "did not stop in time" in capsys.readouterr().err
