import io
from datetime import datetime, timedelta, timezone

from remote_benchmark import stats as stats_module
from remote_benchmark.stats import dump_block_stats


def _dt(seconds):
    return datetime(2026, 1, 1, tzinfo=timezone.utc) + timedelta(seconds=seconds)


def _patch_common(monkeypatch, blocks):
    monkeypatch.setattr(
        stats_module, "get_block_info_cosmos", lambda h, rpc: blocks[h]
    )
    monkeypatch.setattr(stats_module, "mempool_status", lambda rpc: (0, 0))
    monkeypatch.setattr(stats_module, "_get_failed_tx_count", lambda h, rpc: 0)


def test_dump_block_stats_puts_reexecution_and_validation_ratio_in_summary(monkeypatch):
    blocks = {2: (_dt(0), 0), 3: (_dt(1), 5), 4: (_dt(2), 5), 5: (_dt(3), 0)}
    _patch_common(monkeypatch, blocks)
    stm_data = {3: (10, 8), 4: (6, 6)}  # (executed, validated) per block

    summary = dump_block_stats(
        io.StringIO(), "http://rpc", "http://json-rpc", eth=False,
        start=2, end=5, stm_data=stm_data,
    )

    assert summary["avg_reexecution_ratio"] == (10 + 6) / (5 + 5)
    assert summary["avg_validation_ratio"] == (8 + 6) / (10 + 6)


def test_dump_block_stats_reports_none_ratios_without_stm_data(monkeypatch):
    blocks = {2: (_dt(0), 0), 3: (_dt(1), 5), 4: (_dt(2), 0)}
    _patch_common(monkeypatch, blocks)

    summary = dump_block_stats(
        io.StringIO(), "http://rpc", "http://json-rpc", eth=False,
        start=2, end=4, stm_data=None,
    )

    assert summary["avg_reexecution_ratio"] is None
    assert summary["avg_validation_ratio"] is None
