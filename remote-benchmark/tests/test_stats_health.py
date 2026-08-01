from remote_benchmark.stats import (
    _parse_histogram_sum_count,
    _parse_labeled_metric,
    scrape_consensus_health,
    scrape_consensus_health_raw,
    scrape_per_validator_metrics,
)

PROM_TEXT = """\
# HELP cometbft_consensus_rounds Number of rounds.
# TYPE cometbft_consensus_rounds gauge
cometbft_consensus_rounds{chain_id="test"} 2
# HELP cometbft_consensus_round_increment_total Round increments.
# TYPE cometbft_consensus_round_increment_total counter
cometbft_consensus_round_increment_total{chain_id="test",step="Propose"} 5
cometbft_consensus_round_increment_total{chain_id="test",step="Prevote"} 3
# HELP cometbft_consensus_missing_validators Missing validators.
# TYPE cometbft_consensus_missing_validators gauge
cometbft_consensus_missing_validators{chain_id="test"} 1
# HELP cometbft_consensus_byzantine_validators Byzantine validators.
# TYPE cometbft_consensus_byzantine_validators gauge
cometbft_consensus_byzantine_validators{chain_id="test"} 0
# HELP cometbft_consensus_duplicate_block_part Duplicate block parts.
# TYPE cometbft_consensus_duplicate_block_part counter
cometbft_consensus_duplicate_block_part{chain_id="test"} 4
# HELP cometbft_consensus_duplicate_vote Duplicate votes.
# TYPE cometbft_consensus_duplicate_vote counter
cometbft_consensus_duplicate_vote{chain_id="test"} 1
# HELP cometbft_consensus_proposal_receive_count Proposals received.
# TYPE cometbft_consensus_proposal_receive_count counter
cometbft_consensus_proposal_receive_count{chain_id="test",status="accepted"} 20
cometbft_consensus_proposal_receive_count{chain_id="test",status="rejected"} 2
# HELP cometbft_consensus_late_votes Late votes.
# TYPE cometbft_consensus_late_votes counter
cometbft_consensus_late_votes{chain_id="test",vote_type="prevote"} 6
cometbft_consensus_late_votes{chain_id="test",vote_type="precommit"} 1
# HELP cometbft_consensus_block_gossip_parts_received Block gossip parts.
# TYPE cometbft_consensus_block_gossip_parts_received counter
cometbft_consensus_block_gossip_parts_received{chain_id="test",matches_current="true"} 100
cometbft_consensus_block_gossip_parts_received{chain_id="test",matches_current="false"} 7
# HELP cometbft_consensus_validator_missed_blocks Missed blocks per validator.
# TYPE cometbft_consensus_validator_missed_blocks gauge
cometbft_consensus_validator_missed_blocks{chain_id="test",validator_address="AAA"} 3
cometbft_consensus_validator_missed_blocks{chain_id="test",validator_address="BBB"} 0
# HELP cometbft_consensus_validator_last_signed_height Last signed height per validator.
# TYPE cometbft_consensus_validator_last_signed_height gauge
cometbft_consensus_validator_last_signed_height{chain_id="test",validator_address="AAA"} 100
cometbft_consensus_validator_last_signed_height{chain_id="test",validator_address="BBB"} 102
# HELP cometbft_consensus_validator_power Voting power per validator.
# TYPE cometbft_consensus_validator_power gauge
cometbft_consensus_validator_power{chain_id="test",validator_address="AAA"} 10
cometbft_consensus_validator_power{chain_id="test",validator_address="BBB"} 20
"""


def test_scrape_consensus_health_raw_sums_labeled_counters():
    raw = scrape_consensus_health_raw(PROM_TEXT)

    assert raw["round_increments"] == 8  # 5 (Propose) + 3 (Prevote)
    assert raw["duplicate_block_parts"] == 4
    assert raw["duplicate_votes"] == 1
    assert raw["rejected_proposals"] == 2
    assert raw["late_votes"] == 7  # 6 (prevote) + 1 (precommit)
    assert raw["block_gossip_parts_mismatched"] == 7


def test_scrape_consensus_health_raw_none_when_nothing_was_scraped():
    # An all-zero dict is truthy and would pass as a valid baseline, turning the
    # later delta into the node's lifetime total.
    assert scrape_consensus_health_raw("") is None
    assert scrape_consensus_health_raw(None) is None
    assert scrape_consensus_health_raw("# HELP something_else Nothing we read.\n") is None


def test_scrape_consensus_health_raw_found_from_a_partial_scrape():
    partial = 'cometbft_consensus_duplicate_vote{chain_id="test"} 3\n'

    raw = scrape_consensus_health_raw(partial)

    assert raw["duplicate_votes"] == 3
    assert raw["round_increments"] == 0


def test_scrape_consensus_health_reports_zeros_when_nothing_was_scraped():
    health = scrape_consensus_health("")

    assert health["round_increments"] == 0
    assert health["late_votes"] == 0
    assert health["missing_validators"] is None
    assert health["byzantine_validators"] is None


def test_scrape_consensus_health_without_baseline_reports_lifetime_totals():
    health = scrape_consensus_health(PROM_TEXT)

    assert health["round_increments"] == 8
    assert health["current_round"] == 2
    assert health["missing_validators"] == 1
    assert health["byzantine_validators"] == 0


def test_scrape_consensus_health_with_baseline_reports_deltas():
    baseline = {
        "round_increments": 5,
        "duplicate_block_parts": 1,
        "duplicate_votes": 0,
        "rejected_proposals": 1,
        "late_votes": 2,
        "block_gossip_parts_mismatched": 3,
    }

    health = scrape_consensus_health(PROM_TEXT, baseline=baseline)

    assert health["round_increments"] == 3  # 8 - 5
    assert health["rejected_proposals"] == 1  # 2 - 1
    assert health["late_votes"] == 5  # 7 - 2
    assert health["block_gossip_parts_mismatched"] == 4  # 7 - 3
    # point-in-time gauges are unaffected by baseline
    assert health["current_round"] == 2


def test_scrape_per_validator_metrics_groups_by_validator_address():
    per_validator = scrape_per_validator_metrics(PROM_TEXT)

    assert per_validator == {
        "AAA": {"missed_blocks": 3.0, "last_signed_height": 100.0, "power": 10.0},
        "BBB": {"missed_blocks": 0.0, "last_signed_height": 102.0, "power": 20.0},
    }


def test_scrape_per_validator_metrics_empty_when_no_data():
    assert scrape_per_validator_metrics("") == {}


def test_parse_labeled_metric_handles_commas_and_braces_in_label_values():
    lines = [
        'cometbft_consensus_duplicate_vote{chain_id="a,b",note="x}y"} 12',
    ]

    assert _parse_labeled_metric(lines, "cometbft_consensus_duplicate_vote") == [
        ({"chain_id": "a,b", "note": "x}y"}, 12.0)
    ]


def test_parse_labeled_metric_skips_line_with_unterminated_labels():
    assert _parse_labeled_metric(['some_metric{chain_id="a" 1'], "some_metric") == []


def test_parse_histogram_sum_count_accumulates_across_label_sets():
    lines = [
        "# HELP m_seconds help",
        'm_seconds_sum{method="a"} 1.5',
        'm_seconds_count{method="a"} 3',
        'm_seconds_sum{method="b"} 2.5',
        'm_seconds_count{method="b"} 7',
    ]

    assert _parse_histogram_sum_count(lines, "m_seconds") == (4.0, 10)


def test_parse_histogram_sum_count_respects_label_filter():
    lines = [
        'm_seconds_sum{method="a"} 1.5',
        'm_seconds_count{method="a"} 3',
        'm_seconds_sum{method="b"} 2.5',
        'm_seconds_count{method="b"} 7',
    ]

    assert _parse_histogram_sum_count(lines, "m_seconds", 'method="b"') == (2.5, 7)


def test_parse_histogram_sum_count_reports_none_when_metric_absent():
    assert _parse_histogram_sum_count(["other_sum 1"], "m_seconds") == (None, 0)
