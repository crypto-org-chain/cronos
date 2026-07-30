from remote_benchmark.stats import (
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
