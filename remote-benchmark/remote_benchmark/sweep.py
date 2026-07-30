"""Parameter sweeps: apply a config per cell, run bench, and stop early on a
degradation verdict — one run record per cell, plus a summary report.

Config mutation and node restart need host access (ssh/ansible), which this
tool doesn't have — instead each cell runs a pluggable, operator-supplied
shell hook that can do whatever it needs (edit config over ssh, run an
ansible playbook, restart the node, ...), and this module just waits for it
to finish and for the node to come back before benching.
"""

import itertools
import json
import os
import subprocess
import time
from statistics import median

from .results import evaluate_saturation


def load_matrix(data):
    """{apply_config_hook, restart_wait_s, axes: {name: [values]}} -> cells.

    Cells are the cartesian product of every axis, each a {name: value}
    dict, e.g. axes {gas_limit: [30e6, 60e6], workers: [8, 16]} yields four
    cells.
    """
    axes = data.get("axes", {})
    names = list(axes)
    cells = (
        [dict(zip(names, combo)) for combo in itertools.product(*axes.values())]
        if names
        else [{}]
    )
    return {
        "apply_config_hook": data.get("apply_config_hook"),
        "restart_wait_s": data.get("restart_wait_s", 0),
        "cells": cells,
    }


def apply_config(hook_cmd, cell, restart_wait_s=0):
    """Run the operator's apply-config hook for one cell and wait for restart.

    The cell's parameters are passed as JSON in the CELL_PARAMS environment
    variable. Raises subprocess.CalledProcessError if the hook exits non-zero.
    """
    if hook_cmd:
        env = dict(os.environ, CELL_PARAMS=json.dumps(cell))
        subprocess.run(hook_cmd, shell=True, check=True, env=env)
    if restart_wait_s:
        time.sleep(restart_wait_s)


def summarize_sweep(cell_results):
    """One line per cell: params, key metrics, and the saturation verdict."""
    lines = []
    for entry in cell_results:
        params = " ".join(f"{k}={v}" for k, v in entry["cell"].items()) or "(no params)"
        summary = entry["summary"] or {}
        tps = f"{summary['median_tps']:.2f}" if summary.get("multi_block") else "N/A"
        gas_utils = summary.get("gas_utilizations")
        gas_util_pct = f"{median(gas_utils) * 100:.1f}%" if gas_utils else "N/A"
        status = "OK" if entry["ok"] else "FAIL: " + "; ".join(entry["reasons"])
        lines.append(f"{params} | median_tps={tps} gas_util={gas_util_pct} | {status}")
    return "\n".join(lines)


def run_sweep(matrix, run_cell, stop_on_degradation=True):
    """Run every cell in `matrix['cells']`, applying the config hook first.

    run_cell(cell) -> summary dict (as returned by dump_block_stats), used to
    decouple this module from bench's tx-generation/sending machinery so it
    stays testable without an actual chain.

    Returns the list of {cell, summary, ok, reasons} entries. Stops after the
    first cell that fails evaluate_saturation's gates when stop_on_degradation
    is set, leaving the remaining cells unrun.
    """
    results = []
    for cell in matrix["cells"]:
        try:
            apply_config(matrix["apply_config_hook"], cell, matrix["restart_wait_s"])
        except subprocess.CalledProcessError as exc:
            results.append(
                {"cell": cell, "summary": None, "ok": False, "reasons": [f"apply_config_hook failed: {exc}"]}
            )
            if stop_on_degradation:
                break
            continue
        summary = run_cell(cell)
        ok, reasons = evaluate_saturation(summary)
        results.append({"cell": cell, "summary": summary, "ok": ok, "reasons": reasons})
        if not ok and stop_on_degradation:
            break
    return results
