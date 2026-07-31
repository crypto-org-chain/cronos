import json
from types import SimpleNamespace

from remote_benchmark import results as results_module
from remote_benchmark.results import (
    aggregate_summaries,
    build_aggregate_record,
    build_run_record,
    evaluate_saturation,
    fetch_node_fingerprint,
    write_run_record,
)


def _endpoint(**overrides):
    defaults = dict(name="node0", rpc="http://node0", json_rpc="http://node0-evm", node_config={})
    defaults.update(overrides)
    return SimpleNamespace(**defaults)


def _cfg(endpoints):
    return SimpleNamespace(
        endpoints=endpoints,
        model_dump=lambda: {"endpoints": [e.__dict__ for e in endpoints]},
    )


class _FakeResponse:
    def __init__(self, payload):
        self._payload = payload

    def json(self):
        return self._payload


def test_evaluate_saturation_passes_when_all_gates_met():
    summary = {
        "gas_utilizations": [0.95, 0.92],
        "total_failed_txs": 0,
        "total_counted_txs": 100,
        "mempool_min_pending": 5,
    }

    ok, reasons = evaluate_saturation(summary)

    assert ok is True
    assert reasons == []


def test_evaluate_saturation_flags_low_gas_utilization():
    summary = {
        "gas_utilizations": [0.5, 0.4],
        "total_failed_txs": 0,
        "total_counted_txs": 100,
        "mempool_min_pending": 5,
    }

    ok, reasons = evaluate_saturation(summary)

    assert ok is False
    assert "gas utilization" in reasons[0]


def test_evaluate_saturation_flags_high_failure_rate():
    summary = {
        "gas_utilizations": [],
        "total_failed_txs": 5,
        "total_counted_txs": 100,
        "mempool_min_pending": 5,
    }

    ok, reasons = evaluate_saturation(summary)

    assert ok is False
    assert "failed tx rate" in reasons[0]


def test_evaluate_saturation_flags_empty_mempool():
    summary = {
        "gas_utilizations": [],
        "total_failed_txs": 0,
        "total_counted_txs": 100,
        "mempool_min_pending": 0,
    }

    ok, reasons = evaluate_saturation(summary)

    assert ok is False
    assert "mempool pending" in reasons[0]


def test_evaluate_saturation_fails_on_no_load_period():
    ok, reasons = evaluate_saturation(None)

    assert ok is False
    assert "no_load_period" in reasons[0]


def test_fetch_node_fingerprint_uses_declared_config_and_rpc_data(monkeypatch):
    endpoint = _endpoint(node_config={"mempool.type": "app", "libp2p": True})

    def fake_get(url, timeout):
        if url.endswith("/status"):
            return _FakeResponse({"result": {"node_info": {"version": "1.8.0", "network": "test", "moniker": "n0"}}})
        if url.endswith("/abci_info"):
            return _FakeResponse({"result": {"response": {"version": "1.8.0", "data": "app-data"}}})
        if url.endswith("/consensus_params"):
            return _FakeResponse({"result": {"consensus_params": {"block": {"max_gas": "1000000", "max_bytes": "500000"}}}})
        raise AssertionError(f"unexpected url {url}")

    monkeypatch.setattr(results_module.requests, "get", fake_get)

    fingerprint = fetch_node_fingerprint(endpoint)

    assert fingerprint["declared"] == {"mempool.type": "app", "libp2p": True}
    assert fingerprint["node_version"] == "1.8.0"
    assert fingerprint["network"] == "test"
    assert fingerprint["app_version"] == "1.8.0"
    assert fingerprint["block_max_gas"] == "1000000"


def test_fetch_node_fingerprint_degrades_gracefully_when_rpc_unreachable(monkeypatch):
    endpoint = _endpoint()

    def fake_get(_url, timeout):
        raise ConnectionError("unreachable")

    monkeypatch.setattr(results_module.requests, "get", fake_get)

    fingerprint = fetch_node_fingerprint(endpoint)

    assert fingerprint["node_version"] is None
    assert fingerprint["app_version"] is None
    assert "block_max_gas" not in fingerprint


def test_build_run_record_serializes_stall_indices_set(monkeypatch):
    cfg = _cfg([_endpoint()])
    monkeypatch.setattr(results_module, "fetch_node_fingerprint", lambda _endpoint: {"name": "node0"})

    record = build_run_record(
        cfg=cfg,
        config_path="cfg.yaml",
        mode="cosmos",
        load_start=10,
        load_end=20,
        stats_text="block 11 txs=1 2026-07-19T10:00:00+00:00 tps=1.00\n",
        summary={"stall_indices": {2, 1}, "median_tps": 500.0},
        committed_txs=1,
        expected_txs=1,
    )

    assert record["summary"]["stall_indices"] == [1, 2]
    assert record["saturation"] == {"ok": True, "reasons": []}
    assert record["divergence"] is None  # single endpoint, nothing to compare
    assert record["blocks"][0]["height"] == 11
    # must be JSON-serializable, since write_run_record dumps it directly
    json.dumps(record)


def test_write_run_record_creates_parent_dirs_and_writes_json(tmp_path):
    output_path = tmp_path / "nested" / "record.json"

    write_run_record({"a": 1}, output_path)

    assert json.loads(output_path.read_text()) == {"a": 1}


def test_build_run_record_includes_divergence_check_for_multi_node_cfg(monkeypatch):
    cfg = _cfg([_endpoint(name="node0", rpc="http://a"), _endpoint(name="node1", rpc="http://b")])
    monkeypatch.setattr(results_module, "fetch_node_fingerprint", lambda _endpoint: {"name": "node"})
    monkeypatch.setattr(results_module, "collect_heights", lambda _endpoints: {"node0": 20, "node1": 18})
    monkeypatch.setattr(results_module, "height_skew", lambda _heights: 2)
    monkeypatch.setattr(
        results_module,
        "check_app_hash_agreement",
        lambda _endpoints, start, end: [{"height": 15, "hashes": {"node0": "x", "node1": "y"}}],
    )

    record = build_run_record(
        cfg=cfg,
        config_path="cfg.yaml",
        mode="cosmos",
        load_start=10,
        load_end=20,
        stats_text="",
        summary=None,
        committed_txs=0,
        expected_txs=0,
    )

    assert record["divergence"] == {
        "heights": {"node0": 20, "node1": 18},
        "height_skew": 2,
        "app_hash_divergences": [{"height": 15, "hashes": {"node0": "x", "node1": "y"}}],
    }


def test_aggregate_summaries_computes_median_min_max_stdev():
    summaries = [
        {"median_tps": 490.0, "gas_utilizations": [0.9]},
        {"median_tps": 500.0, "gas_utilizations": [0.9]},
        {"median_tps": 510.0, "gas_utilizations": [0.9]},
    ]

    aggregate = aggregate_summaries(summaries)

    assert aggregate["median_tps"]["median"] == 500.0
    assert aggregate["median_tps"]["min"] == 490.0
    assert aggregate["median_tps"]["max"] == 510.0
    assert aggregate["median_tps"]["n"] == 3
    assert aggregate["median_tps"]["stdev"] > 0
    # non-numeric/list metrics are not aggregated
    assert "gas_utilizations" not in aggregate


def test_aggregate_summaries_excludes_no_load_runs():
    summaries = [{"median_tps": 500.0}, None, {"median_tps": 510.0}]

    aggregate = aggregate_summaries(summaries)

    assert aggregate["median_tps"]["n"] == 2


def test_aggregate_summaries_returns_empty_when_all_runs_have_no_load():
    assert aggregate_summaries([None, None]) == {}


def test_aggregate_summaries_excludes_metric_missing_from_any_run():
    summaries = [
        {"median_tps": 500.0},
        {"median_tps": 510.0, "peak_rss": 1024},
        {"median_tps": 520.0, "peak_rss": 2048},
    ]

    aggregate = aggregate_summaries(summaries)

    assert "median_tps" in aggregate
    assert "peak_rss" not in aggregate


def test_build_aggregate_record_reports_num_runs_and_no_load_runs(monkeypatch):
    cfg = _cfg([_endpoint()])
    monkeypatch.setattr(results_module, "fetch_node_fingerprint", lambda _endpoint: {"name": "node0"})

    record = build_aggregate_record(
        cfg=cfg,
        config_path="cfg.yaml",
        summaries=[{"median_tps": 500.0}, None],
    )

    assert record["run_kind"] == "bench-aggregate"
    assert record["num_runs"] == 2
    assert record["no_load_runs"] == 1
    assert record["per_run_saturation"][1]["ok"] is False
    json.dumps(record)
