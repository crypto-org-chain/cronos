import json

from remote_benchmark.sweep import apply_config, load_matrix, run_sweep, summarize_sweep


def test_load_matrix_builds_cartesian_product_of_axes():
    matrix = load_matrix(
        {
            "apply_config_hook": "./apply.sh",
            "restart_wait_s": 5,
            "axes": {"gas_limit": [30_000_000, 60_000_000], "workers": [8, 16]},
        }
    )

    assert matrix["apply_config_hook"] == "./apply.sh"
    assert matrix["restart_wait_s"] == 5
    assert matrix["cells"] == [
        {"gas_limit": 30_000_000, "workers": 8},
        {"gas_limit": 30_000_000, "workers": 16},
        {"gas_limit": 60_000_000, "workers": 8},
        {"gas_limit": 60_000_000, "workers": 16},
    ]


def test_load_matrix_with_no_axes_is_a_single_empty_cell():
    matrix = load_matrix({})

    assert matrix["cells"] == [{}]
    assert matrix["apply_config_hook"] is None
    assert matrix["restart_wait_s"] == 0


def test_apply_config_passes_cell_params_as_json_env_var(tmp_path):
    out_path = tmp_path / "out.json"
    hook = f"echo \"$CELL_PARAMS\" > {out_path}"

    apply_config(hook, {"workers": 16}, restart_wait_s=0)

    assert json.loads(out_path.read_text()) == {"workers": 16}


def test_apply_config_no_hook_is_a_noop():
    apply_config(None, {"workers": 16}, restart_wait_s=0)


def test_run_sweep_stops_after_first_saturation_failure():
    summaries_by_workers = {
        8: {"total_counted_txs": 100, "total_failed_txs": 0, "gas_utilizations": [0.9]},
        16: {"total_counted_txs": 100, "total_failed_txs": 99, "gas_utilizations": [0.9]},
        32: {"total_counted_txs": 100, "total_failed_txs": 0, "gas_utilizations": [0.9]},
    }
    matrix = {"apply_config_hook": None, "restart_wait_s": 0, "cells": [{"workers": w} for w in (8, 16, 32)]}

    results = run_sweep(matrix, lambda cell: summaries_by_workers[cell["workers"]])

    assert len(results) == 2  # stopped after cell 2 (workers=16) failed
    assert results[0]["ok"] is True
    assert results[1]["ok"] is False


def test_run_sweep_without_stop_on_degradation_runs_every_cell():
    matrix = {"apply_config_hook": None, "restart_wait_s": 0, "cells": [{"i": 0}, {"i": 1}]}

    results = run_sweep(matrix, lambda cell: None, stop_on_degradation=False)

    assert len(results) == 2
    assert all(r["ok"] is False for r in results)  # None summary -> no_load_period


def test_run_sweep_stops_on_apply_config_hook_failure():
    matrix = {
        "apply_config_hook": "exit 1",
        "restart_wait_s": 0,
        "cells": [{"i": 0}, {"i": 1}],
    }

    results = run_sweep(matrix, lambda cell: None)

    assert len(results) == 1
    assert results[0]["ok"] is False
    assert "apply_config_hook failed" in results[0]["reasons"][0]


def test_run_sweep_continues_past_apply_config_hook_failure_when_not_stopping():
    calls = []
    matrix = {
        "apply_config_hook": "exit 1",
        "restart_wait_s": 0,
        "cells": [{"i": 0}, {"i": 1}],
    }

    results = run_sweep(matrix, lambda cell: calls.append(cell), stop_on_degradation=False)

    assert len(results) == 2
    assert all(r["ok"] is False for r in results)
    assert calls == []  # run_cell never called since the hook always fails


def test_summarize_sweep_reports_params_metrics_and_status():
    entry_ok = {
        "cell": {"workers": 8},
        "summary": {"multi_block": True, "median_tps": 12.5, "gas_utilizations": [0.8]},
        "ok": True,
        "reasons": [],
    }
    entry_fail = {
        "cell": {"workers": 16},
        "summary": None,
        "ok": False,
        "reasons": ["no_load_period: no transactions observed in the queried range"],
    }

    report = summarize_sweep([entry_ok, entry_fail])

    assert "workers=8" in report
    assert "median_tps=12.50" in report
    assert "gas_util=80.0%" in report
    assert "OK" in report
    assert "workers=16" in report
    assert "FAIL: no_load_period" in report
