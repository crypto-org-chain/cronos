"""Run-record JSON for a `bench` run.

Bundles the config snapshot, a per-node fingerprint, the parsed per-block
series, summary metrics (from stats.py), and a saturation verdict into one
JSON document, so runs can be archived, aggregated (`bench --repeat`), and
diffed (`compare`).
"""

from pathlib import Path
from statistics import median, stdev

import requests
import ujson

from .report import parse_stats

# Saturation gates from the tuning guide: below these, a run measures an
# unsaturated system rather than the throughput ceiling.
GAS_UTILIZATION_MIN_PCT = 90.0
FAILED_TX_MAX_PCT = 1.0


def fetch_node_fingerprint(endpoint):
    """Best-effort per-node fingerprint.

    Node/app version and consensus params come from public RPC. Fields that
    aren't observable over RPC (mempool.type, libp2p enablement, Block-STM
    worker count) are only present if the caller declared them on the
    endpoint's `node_config` — e.g. because a sweep driver just applied that
    config over ssh and knows what it set.
    """
    fingerprint = {
        "name": endpoint.name,
        "rpc": endpoint.rpc,
        "json_rpc": endpoint.json_rpc,
        "declared": dict(getattr(endpoint, "node_config", None) or {}),
    }

    try:
        status = requests.get(f"{endpoint.rpc}/status", timeout=5).json()["result"]
        node_info = status.get("node_info", {})
        fingerprint["node_version"] = node_info.get("version")
        fingerprint["network"] = node_info.get("network")
        fingerprint["moniker"] = node_info.get("moniker")
    except Exception:
        fingerprint["node_version"] = None

    try:
        abci = requests.get(f"{endpoint.rpc}/abci_info", timeout=5).json()
        response = abci.get("result", {}).get("response", {})
        fingerprint["app_version"] = response.get("version")
        fingerprint["app_data"] = response.get("data")
    except Exception:
        fingerprint["app_version"] = None

    try:
        params = requests.get(f"{endpoint.rpc}/consensus_params", timeout=5).json()
        block_params = params["result"]["consensus_params"].get("block", {})
        fingerprint["block_max_gas"] = block_params.get("max_gas")
        fingerprint["block_max_bytes"] = block_params.get("max_bytes")
    except Exception:
        pass

    return fingerprint


def evaluate_saturation(summary):
    """Check the tuning-guide saturation gates against a summary dict
    returned by dump_block_stats/dump_eth_block_stats.

    Returns (ok, reasons). A gate that has no data to evaluate (e.g. eth mode
    has no mempool snapshots) is skipped rather than treated as a failure.
    """
    if summary is None:
        return False, ["no_load_period: no transactions observed in the queried range"]

    reasons = []

    gas_utils = summary.get("gas_utilizations") or []
    if gas_utils:
        median_util_pct = median(gas_utils) * 100
        if median_util_pct < GAS_UTILIZATION_MIN_PCT:
            reasons.append(
                f"median gas utilization {median_util_pct:.1f}% < "
                f"{GAS_UTILIZATION_MIN_PCT:.0f}%"
            )

    total_counted = summary.get("total_counted_txs", 0)
    if total_counted:
        failed_pct = summary["total_failed_txs"] / total_counted * 100
        if failed_pct >= FAILED_TX_MAX_PCT:
            reasons.append(
                f"failed tx rate {failed_pct:.1f}% >= {FAILED_TX_MAX_PCT:.0f}%"
            )

    mempool_min = summary.get("mempool_min_pending")
    if mempool_min is not None and mempool_min <= 0:
        reasons.append("mempool pending txs hit 0 during the load window")

    return len(reasons) == 0, reasons


def _json_safe_summary(summary):
    """stats.py summaries carry a `stall_indices` set, which json can't
    serialize directly."""
    if summary is None:
        return None
    return {
        key: (sorted(value) if isinstance(value, set) else value)
        for key, value in summary.items()
    }


def build_run_record(
    *,
    cfg,
    config_path,
    mode,
    load_start,
    load_end,
    stats_text,
    summary,
    committed_txs,
    expected_txs,
    run_kind="bench",
    extra=None,
):
    """Assemble the full run record for one bench invocation."""
    blocks, text_metrics = parse_stats(stats_text)
    nodes = [fetch_node_fingerprint(endpoint) for endpoint in cfg.endpoints]
    saturation_ok, saturation_reasons = evaluate_saturation(summary)

    record = {
        "run_kind": run_kind,
        "config_path": str(config_path),
        "config": cfg.model_dump(),
        "nodes": nodes,
        "mode": mode,
        "load_period": {"start_height": load_start, "end_height": load_end},
        "committed_txs": committed_txs,
        "expected_txs": expected_txs,
        "blocks": blocks,
        "text_metrics": text_metrics,
        "summary": _json_safe_summary(summary),
        "saturation": {"ok": saturation_ok, "reasons": saturation_reasons},
        "stats_text": stats_text,
    }
    if extra:
        record.update(extra)
    return record


def write_run_record(record, output_path):
    path = Path(output_path)
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(ujson.dumps(record, indent=2))
    return path


def aggregate_summaries(summaries):
    """Aggregate numeric metrics across repeated `bench` runs.

    Returns {metric: {median, min, max, stdev, n}} for every numeric metric
    present in every non-None summary. Runs with no_load_period (summary is
    None) are excluded from the numeric aggregation and counted separately.
    """
    valid = [s for s in summaries if s is not None]
    if not valid:
        return {}

    numeric_keys = [
        key
        for key, value in valid[0].items()
        if isinstance(value, (int, float)) and not isinstance(value, bool)
    ]
    aggregate = {}
    for key in numeric_keys:
        values = [s[key] for s in valid if isinstance(s.get(key), (int, float))]
        if len(values) != len(valid):
            continue
        aggregate[key] = {
            "median": median(values),
            "min": min(values),
            "max": max(values),
            "stdev": stdev(values) if len(values) > 1 else 0.0,
            "n": len(values),
        }
    return aggregate


def build_aggregate_record(*, cfg, config_path, summaries, run_kind="bench-aggregate"):
    """Assemble an aggregate record summarizing repeated `bench` runs."""
    per_run_saturation = [evaluate_saturation(summary) for summary in summaries]
    nodes = [fetch_node_fingerprint(endpoint) for endpoint in cfg.endpoints]

    return {
        "run_kind": run_kind,
        "config_path": str(config_path),
        "config": cfg.model_dump(),
        "nodes": nodes,
        "num_runs": len(summaries),
        "no_load_runs": sum(1 for summary in summaries if summary is None),
        "aggregate": aggregate_summaries(summaries),
        "per_run_saturation": [
            {"ok": ok, "reasons": reasons} for ok, reasons in per_run_saturation
        ],
    }
