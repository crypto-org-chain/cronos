from remote_benchmark.compare import (
    build_comparison,
    compare_metrics,
    diff_config,
    extract_metrics,
    render_comparison_html,
    render_comparison_text,
)


def test_extract_metrics_from_single_run_record():
    record = {
        "run_kind": "bench",
        "summary": {
            "median_tps": 500.0,
            "overall_tps": 480.0,
            "stall_indices": [1, 2],  # non-numeric, must be dropped
            "gas_utilizations": [0.9, 0.95],  # excluded list metric
        },
    }

    metrics = extract_metrics(record)

    assert metrics == {
        "median_tps": {"value": 500.0, "stdev": None, "n": 1},
        "overall_tps": {"value": 480.0, "stdev": None, "n": 1},
    }


def test_extract_metrics_from_aggregate_record():
    record = {
        "run_kind": "bench-aggregate",
        "aggregate": {
            "median_tps": {"median": 510.0, "min": 490.0, "max": 530.0, "stdev": 12.0, "n": 3},
        },
    }

    metrics = extract_metrics(record)

    assert metrics == {"median_tps": {"value": 510.0, "stdev": 12.0, "n": 3}}


def test_compare_metrics_reports_delta_and_pct_change():
    a = {"median_tps": {"value": 500.0, "stdev": None, "n": 1}}
    b = {"median_tps": {"value": 600.0, "stdev": None, "n": 1}}

    rows = compare_metrics(a, b)

    assert rows == [
        {
            "metric": "median_tps",
            "a": 500.0,
            "b": 600.0,
            "delta": 100.0,
            "pct_change": 20.0,
            "significant": None,
        }
    ]


def test_compare_metrics_flags_significance_when_spreads_dont_overlap():
    a = {"median_tps": {"value": 500.0, "stdev": 5.0, "n": 3}}
    b = {"median_tps": {"value": 600.0, "stdev": 5.0, "n": 3}}

    rows = compare_metrics(a, b)

    assert rows[0]["significant"] is True


def test_compare_metrics_flags_not_significant_when_spreads_overlap():
    a = {"median_tps": {"value": 500.0, "stdev": 50.0, "n": 3}}
    b = {"median_tps": {"value": 510.0, "stdev": 50.0, "n": 3}}

    rows = compare_metrics(a, b)

    assert rows[0]["significant"] is False


def test_compare_metrics_only_includes_shared_metrics():
    a = {"median_tps": {"value": 500.0, "stdev": None, "n": 1}, "only_a": {"value": 1, "stdev": None, "n": 1}}
    b = {"median_tps": {"value": 600.0, "stdev": None, "n": 1}, "only_b": {"value": 2, "stdev": None, "n": 1}}

    rows = compare_metrics(a, b)

    assert [row["metric"] for row in rows] == ["median_tps"]


def test_diff_config_reports_only_changed_keys():
    config_a = {"gas_price": 1000000000, "num_txs": 1, "endpoints": [{"name": "a"}]}
    config_b = {"gas_price": 2000000000, "num_txs": 1, "endpoints": [{"name": "b"}]}

    rows = diff_config(config_a, config_b)

    assert rows == [
        {"key": "endpoints[0].name", "a": "a", "b": "b"},
        {"key": "gas_price", "a": 1000000000, "b": 2000000000},
    ]


def test_build_comparison_combines_metrics_and_config_diff():
    record_a = {
        "run_kind": "bench",
        "summary": {"median_tps": 500.0},
        "config": {"gas_price": 1},
    }
    record_b = {
        "run_kind": "bench",
        "summary": {"median_tps": 600.0},
        "config": {"gas_price": 2},
    }

    comparison = build_comparison(record_a, record_b, "a.json", "b.json")

    assert comparison["label_a"] == "a.json"
    assert comparison["label_b"] == "b.json"
    assert comparison["metrics"][0]["metric"] == "median_tps"
    assert comparison["config_diff"] == [{"key": "gas_price", "a": 1, "b": 2}]


def test_render_comparison_text_includes_metrics_and_config_diff():
    comparison = build_comparison(
        {"run_kind": "bench", "summary": {"median_tps": 500.0}, "config": {"gas_price": 1}},
        {"run_kind": "bench", "summary": {"median_tps": 600.0}, "config": {"gas_price": 2}},
        "a.json",
        "b.json",
    )

    text = render_comparison_text(comparison)

    assert "median_tps" in text
    assert "+20.0%" in text
    assert "n/a (single run)" in text
    assert "gas_price: 1 -> 2" in text


def test_render_comparison_html_escapes_and_includes_labels():
    comparison = build_comparison(
        {"run_kind": "bench", "summary": {"median_tps": 500.0}, "config": {}},
        {"run_kind": "bench", "summary": {"median_tps": 600.0}, "config": {}},
        "<a>.json",
        "b.json",
    )

    report = render_comparison_html(comparison)

    assert "&lt;a&gt;.json" in report
    assert "median_tps" in report
    assert "No configuration differences." in report
