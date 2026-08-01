from datetime import datetime, timedelta, timezone
from statistics import median

from remote_benchmark.results import evaluate_saturation
from remote_benchmark.stats import _analyze_load_window, _percentile


def _dt(seconds):
    return datetime(2026, 1, 1, tzinfo=timezone.utc) + timedelta(seconds=seconds)


def test_percentile_of_empty_list():
    assert _percentile([], 95) == 0


def test_percentile_of_single_value():
    assert _percentile([0.4], 95) == 0.4
    assert _percentile([0.4], 0) == 0.4


def test_percentile_of_two_values_interpolates_between_them():
    assert _percentile([1.0, 2.0], 0) == 1.0
    assert _percentile([1.0, 2.0], 50) == 1.5
    assert _percentile([1.0, 2.0], 100) == 2.0
    assert _percentile([2.0, 1.0], 95) == 1.95  # unsorted input is sorted first


def test_percentile_picks_exact_element_on_boundaries():
    values = [1.0, 2.0, 3.0, 4.0, 5.0]
    assert _percentile(values, 0) == 1.0
    assert _percentile(values, 25) == 2.0
    assert _percentile(values, 100) == 5.0


def test_analyze_load_window_excludes_stalled_block_from_gas_rates():
    # Blocks 0 and 1 share a timestamp, so that interval has no rate at all;
    # the last interval is a 27s stall. Both must be dropped from the gas-rate
    # summary, which only works if gps samples stay index-aligned with the
    # block-time samples the stall detector indexes.
    timestamps = [0, 0, 1, 2, 3, 30]
    blocks = [(0, _dt(timestamps[0]))] + [(10, _dt(t)) for t in timestamps[1:]]
    gas_data = [(gu, 100) for gu in (0, 10, 20, 30, 40, 50)]

    summary = _analyze_load_window(blocks, gas_data, per_tx_gas_values=[])

    assert summary["stall_indices"] == {4}
    assert summary["peak_gps"] == 40.0
    assert summary["median_gps"] == 30.0


def test_analyze_load_window_keeps_empty_blocks_in_gas_utilization():
    # A chain alternating full and empty blocks: dropping the zero-gas blocks
    # left the gate looking only at the full ones, so a half-idle chain read as
    # fully saturated.
    tx_counts = [10, 0, 10, 0, 0, 10, 0, 10]
    blocks = [(txs, _dt(i)) for i, txs in enumerate(tx_counts)]
    gas_data = [(95 if txs else 0, 100) for txs in tx_counts]

    summary = _analyze_load_window(blocks, gas_data, per_tx_gas_values=[])

    assert median(summary["gas_utilizations"]) == 0.475
    # what the old gu > 0 filter measured, and why it passed
    full_blocks_only = [u for u in summary["gas_utilizations"] if u > 0]
    assert median(full_blocks_only) == 0.95

    ok, reasons = evaluate_saturation(summary)

    assert ok is False
    assert "median gas utilization 47.5%" in reasons[0]


def test_analyze_load_window_drops_unmeasured_gas_from_utilization():
    # A block whose gas query failed reads as (0, 0); a zero limit means "never
    # measured", not "empty", so it stays out of the sample.
    blocks = [(10, _dt(i)) for i in range(3)]
    gas_data = [(90, 100), (0, 0), (90, 100)]

    summary = _analyze_load_window(blocks, gas_data, per_tx_gas_values=[])

    assert summary["gas_utilizations"] == [0.9, 0.9]
