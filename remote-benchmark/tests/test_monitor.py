import threading

from remote_benchmark import monitor as monitor_module
from remote_benchmark.monitor import BlockSTMMonitor, MempoolMonitor


def _drive(monkeypatch, heights):
    """Feed `heights` to the monitor's block_height and stop it once they run
    out, so _poll can be called directly instead of racing a real thread."""
    remaining = list(heights)

    def fake_block_height(_rpc):
        if not remaining:
            raise AssertionError("polled past the scripted heights")
        return remaining.pop(0)

    monkeypatch.setattr(monitor_module, "block_height", fake_block_height)
    return remaining


def test_start_and_stop_run_the_poll_loop_on_a_joinable_thread(monkeypatch):
    sampled = threading.Event()

    def fake_mempool_status(_rpc):
        sampled.set()
        return (3, 300)

    monkeypatch.setattr(monitor_module, "block_height", lambda _rpc: 7)
    monkeypatch.setattr(monitor_module, "mempool_status", fake_mempool_status)

    monitor = MempoolMonitor("http://rpc", interval=0)
    monitor.start()
    assert sampled.wait(timeout=5), "poll loop never ran on the started thread"
    monitor.stop()

    assert monitor._thread is not None
    assert not monitor._thread.is_alive()
    assert monitor.data == {7: (3, 300)}


def test_data_returns_a_copy_so_later_samples_cannot_mutate_a_taken_snapshot():
    monitor = MempoolMonitor("http://rpc")
    monitor._data[7] = (1, 100)

    snapshot = monitor.data
    monitor._data[8] = (2, 200)

    assert snapshot == {7: (1, 100)}


def test_mempool_monitor_keeps_the_peak_seen_at_a_height(monkeypatch):
    remaining = _drive(monkeypatch, [7, 7, 7])
    samples = [(5, 500), (9, 400), (2, 900)]
    monkeypatch.setattr(monitor_module, "mempool_status", lambda _rpc: samples.pop(0))

    monitor = MempoolMonitor("http://rpc", interval=0)
    monkeypatch.setattr(monitor._stop, "is_set", lambda: not remaining)
    monitor._poll()

    # txs and bytes peak independently - neither sample dominated the other.
    assert monitor.data == {7: (9, 900)}


def test_mempool_monitor_fetches_txpool_once_per_height(monkeypatch):
    remaining = _drive(monkeypatch, [7, 7, 8])
    calls = []

    def fake_txpool_status(json_rpc):
        calls.append(json_rpc)
        return (len(calls), 0)

    monkeypatch.setattr(monitor_module, "txpool_status", fake_txpool_status)
    monkeypatch.setattr(monitor_module, "mempool_status", lambda _rpc: (0, 0))

    monitor = MempoolMonitor("http://rpc", json_rpc="http://evm", interval=0)
    monkeypatch.setattr(monitor._stop, "is_set", lambda: not remaining)
    monitor._poll()

    assert calls == ["http://evm", "http://evm"]
    # height 7's second tick reuses the first sample rather than re-walking the
    # app mempool; height 8 is a new height so it fetches again.
    assert monitor.data == {7: (1, 0), 8: (2, 0)}


def test_mempool_monitor_survives_a_failing_rpc(monkeypatch):
    remaining = _drive(monkeypatch, [7, 7])
    calls = []

    def flaky_mempool_status(_rpc):
        calls.append(1)
        if len(calls) == 1:
            raise RuntimeError("connection reset")
        return (4, 400)

    monkeypatch.setattr(monitor_module, "mempool_status", flaky_mempool_status)

    monitor = MempoolMonitor("http://rpc", interval=0)
    monkeypatch.setattr(monitor._stop, "is_set", lambda: not remaining)
    monitor._poll()

    assert monitor.data == {7: (4, 400)}


def test_blockstm_monitor_scrapes_only_when_the_height_advances(monkeypatch):
    remaining = _drive(monkeypatch, [7, 7, 8])
    scrapes = []

    monkeypatch.setattr(monitor_module, "_fetch_prometheus", lambda t: t)
    monkeypatch.setattr(
        monitor_module,
        "scrape_blockstm_metrics",
        lambda text: scrapes.append(text)
        or {"executed_txs": len(scrapes), "validated_txs": 10 * len(scrapes)},
    )

    monitor = BlockSTMMonitor("http://rpc", "http://telemetry", interval=0)
    monkeypatch.setattr(monitor._stop, "is_set", lambda: not remaining)
    monitor._poll()

    assert scrapes == ["http://telemetry"] * 2
    assert monitor.data == {7: (1, 10), 8: (2, 20)}


def test_blockstm_monitor_skips_a_height_with_no_executed_txs(monkeypatch):
    remaining = _drive(monkeypatch, [7])
    monkeypatch.setattr(monitor_module, "_fetch_prometheus", lambda t: t)
    monkeypatch.setattr(
        monitor_module,
        "scrape_blockstm_metrics",
        lambda _text: {"executed_txs": 0, "validated_txs": 0},
    )

    monitor = BlockSTMMonitor("http://rpc", "http://telemetry", interval=0)
    monkeypatch.setattr(monitor._stop, "is_set", lambda: not remaining)
    monitor._poll()

    # A zero gauge means the executor never ran for that block, not that it ran
    # with nothing to do - recording it would drag the per-block average down.
    assert monitor.data == {}
