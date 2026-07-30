from datetime import datetime, timedelta, timezone

from remote_benchmark import soak as soak_module
from remote_benchmark.soak import (
    LEAK_RSS_SLOPE_BYTES_PER_S,
    _checkpoint,
    fit_trends,
    soak_verdict,
)


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


def test_fit_trends_detects_growing_rss():
    checkpoints = [
        {"elapsed_s": 0, "tps": 10.0, "avg_block_time_ms": 500.0, "rss_bytes": 100_000_000},
        {"elapsed_s": 10, "tps": 10.0, "avg_block_time_ms": 500.0, "rss_bytes": 100_000_000 + 20 * 1024 * 1024},
        {"elapsed_s": 20, "tps": 10.0, "avg_block_time_ms": 500.0, "rss_bytes": 100_000_000 + 40 * 1024 * 1024},
    ]

    trends = fit_trends(checkpoints)

    assert trends["rss_bytes"] > LEAK_RSS_SLOPE_BYTES_PER_S
    assert trends["avg_block_time_ms"] == 0.0
    assert trends["tps"] == 0.0


def test_fit_trends_returns_none_for_metric_with_insufficient_samples():
    checkpoints = [{"elapsed_s": 0, "tps": 1.0, "avg_block_time_ms": None, "rss_bytes": None}]

    trends = fit_trends(checkpoints)

    assert trends["avg_block_time_ms"] is None
    assert trends["rss_bytes"] is None


def test_soak_verdict_flags_rss_leak():
    verdict = soak_verdict({"rss_bytes": 2 * 1024 * 1024, "avg_block_time_ms": 0.0, "tps": 0.0})

    assert verdict["ok"] is False
    assert "memory leak" in verdict["reasons"][0]


def test_soak_verdict_flags_block_time_degradation():
    verdict = soak_verdict({"rss_bytes": 0.0, "avg_block_time_ms": 5.0, "tps": -1.0})

    assert verdict["ok"] is False
    assert "degradation" in verdict["reasons"][0]


def test_soak_verdict_ok_when_stable():
    verdict = soak_verdict({"rss_bytes": 0.0, "avg_block_time_ms": 0.0, "tps": 0.0})

    assert verdict == {"ok": True, "reasons": []}
