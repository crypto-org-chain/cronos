"""Run-record JSON for a `bench` run.

Bundles the config snapshot, a per-node fingerprint, the parsed per-block
series, summary metrics (from stats.py), and a saturation verdict into one
JSON document, so runs can be archived, aggregated (`bench --repeat`), and
diffed (`compare`).
"""

import time
from pathlib import Path
from statistics import median, stdev

import requests
import ujson

from .divergence import check_app_hash_agreement, collect_heights, height_skew
from .report import parse_stats

# Saturation gates from the tuning guide: below these, a run measures an
# unsaturated system rather than the throughput ceiling.
GAS_UTILIZATION_MIN_PCT = 90.0
FAILED_TX_MAX_PCT = 1.0

# A node further behind than this is not following consensus; a few blocks of
# skew is normal under load. Sized for a ~1s block time, where 50 blocks is a
# ~50s margin; a devnet tuned for throughput (timeout_commit 100-200ms) buys
# only 5-10s from the same number, which a GC pause or a compaction stall can
# reach on a healthy node — hence the shrinking-gap resample below.
MAX_HEIGHT_SKEW_BLOCKS = 50
# Long enough for the laggard to commit at least one block at any plausible
# block time, which is all the resample needs to tell catching-up from stuck.
SKEW_RESAMPLE_DELAY_S = 1.0


def _safe_get(url):
    """GET a CometBFT RPC endpoint's JSON body, or {} on any failure."""
    try:
        return requests.get(url, timeout=5).json()
    except Exception:
        return {}


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

    status = _safe_get(f"{endpoint.rpc}/status")
    node_info = (status.get("result") or {}).get("node_info") or {}
    fingerprint["node_version"] = node_info.get("version")
    fingerprint["network"] = node_info.get("network")
    fingerprint["moniker"] = node_info.get("moniker")

    abci = _safe_get(f"{endpoint.rpc}/abci_info")
    response = (abci.get("result") or {}).get("response") or {}
    fingerprint["app_version"] = response.get("version")
    fingerprint["app_data"] = response.get("data")

    params = _safe_get(f"{endpoint.rpc}/consensus_params")
    if params.get("result") is not None:
        block_params = (params.get("result") or {}).get("consensus_params") or {}
        block_params = block_params.get("block") or {}
        fingerprint["block_max_gas"] = block_params.get("max_gas")
        fingerprint["block_max_bytes"] = block_params.get("max_bytes")

    return fingerprint


def evaluate_saturation(summary):
    """Check the tuning-guide saturation gates against a summary dict
    returned by dump_block_stats/dump_eth_block_stats.

    Returns (ok, reasons). A gate that has no data to evaluate (e.g. eth mode
    has no mempool snapshots) is skipped, but a run where *no* gate had data
    measured nothing and fails — an unmeasurable run must not read as healthy.
    """
    if summary is None:
        return False, ["no_load_period: no transactions observed in the queried range"]

    reasons = []
    gates_evaluated = 0

    gas_utils = summary.get("gas_utilizations") or []
    if gas_utils:
        gates_evaluated += 1
        median_util_pct = median(gas_utils) * 100
        if median_util_pct < GAS_UTILIZATION_MIN_PCT:
            reasons.append(
                f"median gas utilization {median_util_pct:.1f}% < "
                f"{GAS_UTILIZATION_MIN_PCT:.0f}%"
            )

    total_counted = summary.get("total_counted_txs")
    total_failed = summary.get("total_failed_txs")
    # Either key absent means the failure count was never measured, which is not
    # the same as a measured zero: the gate has no data and is skipped.
    if total_counted and total_failed is not None:
        gates_evaluated += 1
        failed_pct = total_failed / total_counted * 100
        if failed_pct >= FAILED_TX_MAX_PCT:
            reasons.append(
                f"failed tx rate {failed_pct:.1f}% >= {FAILED_TX_MAX_PCT:.0f}%"
            )

    mempool_min = summary.get("mempool_min_pending")
    if mempool_min is not None:
        gates_evaluated += 1
        if mempool_min <= 0:
            reasons.append("mempool pending txs hit 0 during the load window")

    if not gates_evaluated:
        return False, ["no gates had data - saturation unmeasured"]

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


def check_divergence(endpoints):
    """Multi-node state-divergence check: height skew plus a forward-sampled
    app-hash agreement check. None when there's only one endpoint to check.

    App hashes are sampled forward from now rather than read back over the load
    window: a node only reports the hash it computed for its own tip, and past
    per-node hashes aren't retrievable after the fact.

    A skew over the threshold is resampled once, and what the resample asks is
    whether the gap *shrank*, not whether it is still over the threshold: an
    already-accumulated 50-block gap cannot close within the resample delay, so
    a node recovering from a GC pause or catching up after a restart would
    otherwise still read as confirmed divergence. A gap that shrank means the
    laggard is committing faster than the leader, i.e. catching up; only a gap
    that held or widened is a node that stopped following consensus.
    """
    if len(endpoints) < 2:
        return None
    heights = collect_heights(endpoints)
    skew = height_skew(heights)
    resampled_heights = None
    catching_up = False
    dropped_on_resample = None
    if skew is not None and skew > MAX_HEIGHT_SKEW_BLOCKS:
        time.sleep(SKEW_RESAMPLE_DELAY_S)
        resampled_heights = collect_heights(endpoints)
        resampled_skew = height_skew(resampled_heights)
        if resampled_skew is None:
            dropped_on_resample = [
                name
                for name, height in heights.items()
                if height is not None and resampled_heights.get(name) is None
            ]
        else:
            catching_up = (
                resampled_skew > MAX_HEIGHT_SKEW_BLOCKS and resampled_skew < skew
            )
        skew = resampled_skew

    divergences = check_app_hash_agreement(endpoints)
    if dropped_on_resample is not None:
        divergences.append(
            {
                "kind": "unverified",
                "unreachable": dropped_on_resample,
                "reason": f"height skew unmeasurable on the resample: fewer than two "
                f"endpoints answered /status ({resampled_heights}); "
                f"{dropped_on_resample} dropped out between samples, so the first "
                f"sample's skew of {height_skew(heights)} could not be confirmed",
            }
        )
    elif skew is None:
        divergences.append(
            {
                "kind": "unverified",
                "unreachable": [
                    name for name, height in heights.items() if height is None
                ],
                "reason": f"height skew unmeasurable: fewer than two endpoints "
                f"answered /status ({heights})",
            }
        )
    if catching_up:
        divergences.append(
            {
                "kind": "unverified",
                "reason": f"height skew {skew} blocks > {MAX_HEIGHT_SKEW_BLOCKS} but "
                f"shrinking between samples ({heights} then {resampled_heights}) — the "
                "lagging node is catching up, not diverged",
            }
        )
    record = {
        "heights": heights,
        "height_skew": skew,
        "app_hash_divergences": divergences,
    }
    if resampled_heights is not None:
        record["resampled_heights"] = resampled_heights
    if catching_up:
        record["height_skew_catching_up"] = True
    return record


def _split_divergence_entries(divergence):
    """(confirmed, unverified) reason lists from the app-hash entries.

    Entries without a `kind` are treated as confirmed: a caller that hand-builds
    an entry means a finding, and defaulting the other way would silently
    downgrade it to a warning.
    """
    confirmed = []
    unverified = []
    for entry in divergence.get("app_hash_divergences") or []:
        reason = entry.get("reason") or str(entry)
        bucket = unverified if entry.get("kind") == "unverified" else confirmed
        bucket.append(reason)
    return confirmed, unverified


def divergence_reasons(divergence):
    """Confirmed-divergence reasons from a check_divergence result, empty when
    the nodes agreed and kept pace.

    Only outcomes that actually establish a mismatch land here; an unreachable or
    slow node means agreement could not be verified, which divergence_warnings
    reports instead of aborting the run.

    None (single endpoint, nothing to compare) yields no reasons: a one-node
    run can't diverge, so it must not fail the check.
    """
    if not divergence:
        return []
    reasons, _ = _split_divergence_entries(divergence)
    skew = divergence.get("height_skew")
    if (
        skew is not None
        and skew > MAX_HEIGHT_SKEW_BLOCKS
        and not divergence.get("height_skew_catching_up")
    ):
        reasons.append(
            f"height skew {skew} blocks > {MAX_HEIGHT_SKEW_BLOCKS} and not shrinking "
            f"on a resample: "
            f"{divergence.get('resampled_heights') or divergence.get('heights')}"
        )
    return reasons


def divergence_warnings(divergence):
    """Reasons the divergence check couldn't verify agreement — a node that never
    answered, a chain that didn't advance, no height compared by two nodes, a
    laggard whose gap is closing.

    Reported rather than raised: none of these observed a mismatch, so aborting
    on them would fail runs whose only problem was an unreachable or slow node.
    """
    if not divergence:
        return []
    _, warnings = _split_divergence_entries(divergence)
    return warnings


def consensus_health_reasons(summary):
    """Byzantine validators seen during the load window.

    A consensus-safety violation, not a tuning signal, so it belongs with the
    divergence failures rather than the opt-in saturation gates: a correctly
    functioning validator set never reports byzantine behaviour, and the count
    is a baseline-relative delta over the load window rather than a lifetime
    total, so any increment happened during this run.
    """
    if not summary:
        return []
    count = summary.get("byzantine_validators")
    if not count:
        return []
    return [f"{count:.0f} byzantine validator(s) reported during the load window"]


def consensus_health_warnings(summary):
    """Missing validators seen in the sampled block.

    Warned rather than raised: unlike the byzantine counter this is a
    point-in-time gauge for a single block, so one validator missing a precommit
    under saturation load — or a deliberately tiny-stake validator that is
    expected to be offline — reads the same as a fault. Neither breaks
    consensus, so it must not abort the run.
    """
    if not summary:
        return []
    count = summary.get("missing_validators")
    if not count:
        return []
    return [
        f"{count:.0f} missing validator(s) in the sampled block — a missed "
        "precommit is expected under load and does not break consensus"
    ]


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
    divergence=None,
    extra=None,
):
    """Assemble the full run record for one bench invocation.

    `divergence` lets a caller that already ran check_divergence (to gate on it)
    reuse that result instead of paying for a second forward sampling window.
    """
    blocks, text_metrics = parse_stats(stats_text)
    nodes = [fetch_node_fingerprint(endpoint) for endpoint in cfg.endpoints]
    saturation_ok, saturation_reasons = evaluate_saturation(summary)
    if divergence is None:
        divergence = check_divergence(cfg.endpoints)

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
        "divergence": divergence,
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

    all_keys = {key for s in valid for key in s}
    numeric_keys = [
        key
        for key in all_keys
        if all(
            isinstance(s.get(key), (int, float)) and not isinstance(s.get(key), bool)
            for s in valid
        )
    ]
    aggregate = {}
    for key in numeric_keys:
        values = [s[key] for s in valid]
        aggregate[key] = {
            "median": median(values),
            "min": min(values),
            "max": max(values),
            "stdev": stdev(values) if len(values) > 1 else 0.0,
            "n": len(values),
        }
    return aggregate


def build_aggregate_record(
    *, cfg, config_path, summaries, divergences=None, run_kind="bench-aggregate"
):
    """Assemble an aggregate record summarizing repeated `bench` runs.

    `divergences` is the per-run check_divergence result, kept per run rather
    than merged: a divergence in one run is a finding about that run, and
    collapsing them would hide which run diverged.
    """
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
        "per_run_divergence": list(divergences or []),
    }
