"""CometBFT/Cosmos SDK Prometheus scrapers: consensus timing, consensus
health, per-validator gauges, and Block-STM gauges.

Built on the generic parsing primitives in promtext.py.
"""

from .promtext import (
    labeled_metric_by,
    parse_histogram_sum_count,
    parse_labeled_metric,
)


def scrape_blockstm_metrics(prom_text):
    """Parse block-stm gauges from Prometheus text."""
    result = {}
    for line in prom_text.splitlines():
        if line.startswith("#"):
            continue
        if "blockstm_executed_txs" in line:
            result["executed_txs"] = float(line.split()[-1])
        elif "blockstm_validated_txs" in line:
            result["validated_txs"] = float(line.split()[-1])
    return result


# (key, metric_name, label_filter) for every cumulative consensus histogram.
# Cumulative _sum/_count grow over the whole node process lifetime, so to scope
# an average to the load period we snapshot raw (sum, count) at load start and
# subtract — see scrape_consensus_raw / the baseline arg of
# scrape_consensus_metrics.
_CONSENSUS_HISTOGRAMS = [
    *[
        (
            f"step_{step}",
            "cometbft_consensus_step_duration_seconds",
            f'step="{step}"',
        )
        for step in (
            "NewHeight",
            "NewRound",
            "Propose",
            "Prevote",
            "PrevoteWait",
            "Precommit",
            "PrecommitWait",
            "Commit",
        )
    ],
    ("finalize_block_ms", "cometbft_state_block_processing_time", None),
    (
        "abci_finalize_block",
        "cometbft_abci_connection_method_timing_seconds",
        'method="finalize_block"',
    ),
    (
        "abci_commit",
        "cometbft_abci_connection_method_timing_seconds",
        'method="commit"',
    ),
    ("block_interval", "cometbft_consensus_block_interval_seconds", None),
]


def scrape_consensus_raw(prom_text):
    """Snapshot raw cumulative (sum, count) for each consensus histogram.

    Pass the result as the `baseline` arg of scrape_consensus_metrics after the
    load finishes to get load-period averages instead of lifetime ones.
    """
    lines = prom_text.splitlines()
    raw = {}
    for key, metric, label in _CONSENSUS_HISTOGRAMS:
        total, count = parse_histogram_sum_count(lines, metric, label)
        if total is not None:
            raw[key] = (total, count)
    return raw


# Cumulative counters behind the consensus-health section: round changes
# (timeouts forcing a new round), rejected/late proposals and votes,
# duplicate/mismatched block-part gossip. Snapshotted at load start and
# subtracted, same rationale as _CONSENSUS_HISTOGRAMS.
_CONSENSUS_HEALTH_COUNTERS = [
    ("round_increments", "cometbft_consensus_round_increment_total"),
    ("duplicate_block_parts", "cometbft_consensus_duplicate_block_part"),
    ("duplicate_votes", "cometbft_consensus_duplicate_vote"),
]


def scrape_consensus_health_raw(prom_text):
    """Snapshot raw cumulative consensus-health counters (see
    scrape_consensus_health for the baseline-relative view).

    Returns None when the text carries none of the counters (telemetry
    unreachable, or a scrape that raced node startup): an all-zero dict is
    truthy and would pass as a valid baseline, turning the later "delta" into
    the node's lifetime total.
    """
    lines = (prom_text or "").splitlines()
    raw = {}
    found = False

    def _sum(metric, predicate=None):
        nonlocal found
        samples = parse_labeled_metric(lines, metric)
        found = found or bool(samples)
        return sum(
            value for labels, value in samples if predicate is None or predicate(labels)
        )

    for key, metric in _CONSENSUS_HEALTH_COUNTERS:
        raw[key] = _sum(metric)
    raw["rejected_proposals"] = _sum(
        "cometbft_consensus_proposal_receive_count",
        lambda labels: labels.get("status") == "rejected",
    )
    raw["late_votes"] = _sum("cometbft_consensus_late_votes")
    raw["block_gossip_parts_mismatched"] = _sum(
        "cometbft_consensus_block_gossip_parts_received",
        lambda labels: labels.get("matches_current") == "false",
    )
    return raw if found else None


def scrape_consensus_health(prom_text, baseline=None):
    """Multi-round rate, missed-proposal/vote counters, and block-part
    mismatch counters, plus point-in-time missing/byzantine validator gauges.

    `round_increments` is the clearest "multi-round rate" signal: CometBFT
    only increments the round when the current one times out without
    reaching a decision, so a nonzero count over the load period means the
    network needed extra rounds (missed proposals, slow votes, etc.) to
    finalize some block.

    With `baseline` (a scrape_consensus_health_raw snapshot from load start),
    counters are deltas over the load period; without it, they're lifetime
    totals. A baseline that measured nothing is None, so it is skipped rather
    than subtracted as zeros.
    """
    lines = (prom_text or "").splitlines()
    raw = scrape_consensus_health_raw(prom_text)
    if raw is None:
        raw = dict.fromkeys(
            [key for key, _ in _CONSENSUS_HEALTH_COUNTERS]
            + ["rejected_proposals", "late_votes", "block_gossip_parts_mismatched"],
            0,
        )
    elif baseline:
        raw = {key: raw[key] - baseline.get(key, 0) for key in raw}

    result = dict(raw)
    result["current_round"] = parse_labeled_metric(lines, "cometbft_consensus_rounds")
    result["current_round"] = (
        result["current_round"][0][1] if result["current_round"] else None
    )
    result["missing_validators"] = parse_labeled_metric(lines, "cometbft_consensus_missing_validators")
    result["missing_validators"] = (
        result["missing_validators"][0][1] if result["missing_validators"] else None
    )
    result["byzantine_validators"] = parse_labeled_metric(lines, "cometbft_consensus_byzantine_validators")
    result["byzantine_validators"] = (
        result["byzantine_validators"][0][1] if result["byzantine_validators"] else None
    )
    return result


def scrape_per_validator_metrics(prom_text):
    """{validator_address: {missed_blocks, last_signed_height, power}} from
    CometBFT's per-validator gauges. All point-in-time (not baseline-relative)."""
    lines = prom_text.splitlines()
    missed = labeled_metric_by(lines, "cometbft_consensus_validator_missed_blocks", "validator_address")
    signed = labeled_metric_by(
        lines, "cometbft_consensus_validator_last_signed_height", "validator_address"
    )
    power = labeled_metric_by(lines, "cometbft_consensus_validator_power", "validator_address")

    addresses = set(missed) | set(signed) | set(power)
    return {
        addr: {
            "missed_blocks": missed.get(addr),
            "last_signed_height": signed.get(addr),
            "power": power.get(addr),
        }
        for addr in addresses
    }


def scrape_consensus_metrics(prom_text, baseline=None):
    """Parse CometBFT consensus stage timings from Prometheus text.

    Returns dict mapping stage names to (avg_seconds, sample_count).

    When `baseline` (a scrape_consensus_raw snapshot from load start) is given,
    averages are computed over the delta so they reflect only the load period
    rather than the node's whole lifetime.
    """
    lines = prom_text.splitlines()
    result = {}

    for key, metric, label in _CONSENSUS_HISTOGRAMS:
        total, count = parse_histogram_sum_count(lines, metric, label)
        if total is None:
            continue
        if baseline and key in baseline:
            base_total, base_count = baseline[key]
            total -= base_total
            count -= base_count
        if count > 0:
            result[key] = (total / count, count)

    # quorum delays are point-in-time gauges, not cumulative — report current
    # value directly (no baseline delta applies).
    for line in lines:
        if line.startswith("#"):
            continue
        if "cometbft_consensus_quorum_prevote_delay" in line:
            result["quorum_prevote_delay"] = (float(line.split()[-1]), 1)
        elif "cometbft_consensus_quorum_precommit_delay" in line:
            result["quorum_precommit_delay"] = (float(line.split()[-1]), 1)

    return result
