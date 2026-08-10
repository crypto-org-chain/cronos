import io
from datetime import datetime, timedelta, timezone

from remote_benchmark import resources as resources_module
from remote_benchmark import stats as stats_module
from remote_benchmark.results import evaluate_saturation
from remote_benchmark.stats import dump_block_stats


def _dt(seconds):
    return datetime(2026, 1, 1, tzinfo=timezone.utc) + timedelta(seconds=seconds)


def _patch_common(monkeypatch, blocks):
    def fake_blockchain_range(lo, hi, rpc):
        return {h: (blocks[h][1], blocks[h][0].isoformat()) for h in range(lo, hi + 1)}

    monkeypatch.setattr(stats_module, "blockchain_range", fake_blockchain_range)
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


def test_dump_block_stats_excludes_window_edges_from_mempool_min(monkeypatch):
    # Both edges snapshot a drained mempool: the anchor block precedes any load
    # tx, and the last tx block is where the load generator has already stopped
    # and the queue empties. Including either drags mempool_min_pending to 0 and
    # trips the saturation gate on a healthy, fully saturated run.
    blocks = {2: (_dt(0), 0), 3: (_dt(1), 5), 4: (_dt(2), 5), 5: (_dt(3), 5), 6: (_dt(4), 0)}
    _patch_common(monkeypatch, blocks)
    mempool_data = {2: (0, 0), 3: (100, 1000), 4: (50, 500), 5: (0, 0), 6: (0, 0)}

    summary = dump_block_stats(
        io.StringIO(), "http://rpc", "http://json-rpc", eth=False,
        start=2, end=6, mempool_data=mempool_data,
    )

    assert summary["mempool_min_pending"] == 50


def test_dump_block_stats_keeps_the_only_tx_block_in_mempool_min(monkeypatch):
    # A single-tx-block window has no interior left after dropping both edges,
    # so it keeps that block rather than reporting nothing measured.
    blocks = {2: (_dt(0), 0), 3: (_dt(1), 5)}
    _patch_common(monkeypatch, blocks)
    mempool_data = {2: (0, 0), 3: (7, 70)}

    summary = dump_block_stats(
        io.StringIO(), "http://rpc", "http://json-rpc", eth=False,
        start=2, end=3, mempool_data=mempool_data,
    )

    assert summary["mempool_min_pending"] == 7


def test_get_failed_tx_count_returns_none_when_block_results_is_unavailable(monkeypatch):
    def boom(height, rpc):
        raise RuntimeError("rpc unreachable")

    monkeypatch.setattr(stats_module, "block_results", boom)

    assert stats_module._get_failed_tx_count(3, "http://rpc") is None


def test_dump_block_stats_leaves_failed_tx_gate_unevaluated_when_unmeasurable(monkeypatch):
    # A 0 here would read as "0% failures" and let the saturation gate pass on
    # data that was never measured; total_counted_txs must stay 0 instead.
    blocks = {2: (_dt(0), 0), 3: (_dt(1), 5), 4: (_dt(2), 5)}
    _patch_common(monkeypatch, blocks)
    monkeypatch.setattr(stats_module, "_get_failed_tx_count", lambda h, rpc: None)

    summary = dump_block_stats(
        io.StringIO(), "http://rpc", "http://json-rpc", eth=False, start=2, end=4,
    )

    assert summary["total_counted_txs"] == 0
    assert summary["total_failed_txs"] == 0
    assert evaluate_saturation(summary)[1] != []


def test_dump_block_stats_eth_mode_sources_timestamps_from_blockchain_range(monkeypatch):
    # eth=True must not call `block()` per height - timestamps come from the
    # same chunked /blockchain fetch as eth=False, and only gas/tx data comes
    # from a per-height eth_getBlockByNumber call.
    blocks = {2: (_dt(0), 0), 3: (_dt(1), 2), 4: (_dt(2), 0)}
    gas_and_txs = {2: (0, 0, 0), 3: (2, 100, 1000), 4: (0, 0, 0)}
    _patch_common(monkeypatch, blocks)
    monkeypatch.setattr(stats_module, "block", _fail_if_called)
    monkeypatch.setattr(
        stats_module, "_get_block_gas_and_txs", lambda h, json_rpc: gas_and_txs[h]
    )

    summary = dump_block_stats(
        io.StringIO(), "http://rpc", "http://json-rpc", eth=True, start=2, end=4,
    )

    assert summary["total_counted_txs"] == 2
    assert summary["tx_gas_list"] == [50]


def _fail_if_called(*args, **kwargs):
    raise AssertionError("block() should not be called in the eth=True path")


def test_dump_block_stats_reports_disk_net_na_when_the_scrape_fails(monkeypatch):
    blocks = {2: (_dt(0), 0), 3: (_dt(1), 5), 4: (_dt(2), 5)}
    _patch_common(monkeypatch, blocks)
    monkeypatch.setattr(resources_module, "fetch_node_exporter", lambda url: "")

    out = io.StringIO()
    dump_block_stats(
        out, "http://rpc", "http://json-rpc", eth=False, start=2, end=4,
        node_exporter="http://node-exporter",
    )

    assert "disk_net N/A" in out.getvalue()
