from datetime import datetime, timezone

import yaml

from remote_benchmark.report import bucket_by_second, generate_report, parse_stats


def test_parse_stats_extracts_blocks_and_summary_metrics():
    blocks, metrics = parse_stats(
        "block 41 txs=12 2026-07-19T10:00:00+00:00 20ms tps=600.00\n"
        "block 42 txs=8 gas=168000 2026-07-19T10:00:00.020+00:00 "
        "20ms tps=400.00\n"
        "peak_tps 600.00\n"
        "committed_cosmos_txs 20/20\n"
    )

    assert blocks == [
        {
            "height": 41,
            "transactions": 12,
            "gas_consumed": 0,
            "tps": 600.0,
            "timestamp": "2026-07-19T10:00:00+00:00",
        },
        {
            "height": 42,
            "transactions": 8,
            "gas_consumed": 168000,
            "tps": 400.0,
            "timestamp": "2026-07-19T10:00:00.020+00:00",
        },
    ]
    assert metrics["peak_tps"] == "600.00"
    assert metrics["committed_cosmos_txs"] == "20/20"


def test_bucket_by_second_aggregates_blocks_and_fills_idle_seconds():
    blocks, _ = parse_stats(
        "block 41 txs=12 gas=252000 2026-07-19T10:00:00.100+00:00 "
        "20ms tps=600.00\n"
        "block 42 txs=8 gas=168000 2026-07-19T10:00:00.900+00:00 "
        "20ms tps=400.00\n"
        "block 43 txs=10 gas=210000 2026-07-19T10:00:02.100+00:00 "
        "20ms tps=500.00\n"
    )

    assert bucket_by_second(blocks) == [
        {
            "elapsed_second": 0,
            "timestamp": "2026-07-19T10:00:00+00:00",
            "transactions": 20,
            "gas_consumed": 420000,
            "rolling_tps_5s": 20.0,
        },
        {
            "elapsed_second": 1,
            "timestamp": "2026-07-19T10:00:01+00:00",
            "transactions": 0,
            "gas_consumed": 0,
            "rolling_tps_5s": 10.0,
        },
        {
            "elapsed_second": 2,
            "timestamp": "2026-07-19T10:00:02+00:00",
            "transactions": 10,
            "gas_consumed": 210000,
            "rolling_tps_5s": 10.0,
        },
    ]


def test_bucket_by_second_accepts_timestamp_format_emitted_by_stats():
    blocks, _ = parse_stats(
        "block 101 txs=1200 gas=25200000 "
        "2026-07-19 12:00:00.100000+08:00 100ms tps=12000.00\n"
    )

    assert bucket_by_second(blocks) == [
        {
            "elapsed_second": 0,
            "timestamp": "2026-07-19T04:00:00+00:00",
            "transactions": 1200,
            "gas_consumed": 25200000,
            "rolling_tps_5s": 1200.0,
        }
    ]


def test_generate_report_lists_all_params_and_embeds_chart_data(tmp_path):
    config = {
        "endpoints": [
            {
                "name": "node0",
                "rpc": "http://127.0.0.1:26657",
                "json_rpc": "http://127.0.0.1:26651",
            }
        ],
        "chain_id": 777,
        "tx_type": "simple-transfer",
        "send_interval": 0.05,
    }
    config_path = tmp_path / "config.yaml"
    stats_path = tmp_path / "stats.log"
    output_path = tmp_path / "report" / "20260719-120000.html"
    config_path.write_text(yaml.safe_dump(config))
    stats_path.write_text(
        "block 10 txs=0 2026-07-19T04:00:00+00:00 - tps=0.00\n"
        "block 11 txs=25 gas=525000 2026-07-19T04:00:00.020+00:00 "
        "20ms tps=1250.00\n"
        "peak_tps 1250.00\n"
        "total_txs 25\n"
        "committed_cosmos_txs 25/25\n"
    )

    generate_report(
        config_path,
        stats_path,
        output_path,
        datetime(2026, 7, 19, 12, 0, tzinfo=timezone.utc),
        validators=1,
        testcase="simple-transfer",
        start_account=1,
        end_account=8000,
    )

    report = output_path.read_text()
    assert "Parameters" in report
    assert "benchmark.validators" in report
    assert "benchmark.start_account" in report
    assert "benchmark.end_account" in report
    assert "endpoints[0].json_rpc" in report
    assert "send_interval" in report
    assert "Transactions by block" in report
    assert "Gas consumed by block" in report
    assert "Transactions per second" in report
    assert "Gas consumed per second" in report
    assert "Peak 1s TPS" in report
    assert "Peak 5s avg TPS" in report
    assert "Peak gas / second" in report
    assert '"height":11,"transactions":25,"gas_consumed":525000' in report
    assert "'gas_consumed','Gas consumed'" in report
    assert 'const secondData=[{"elapsed_second":0' in report
    assert "5-second moving average" in report
    assert "committed_cosmos_txs" not in report
    assert "25/25" in report
    assert report.count('class="field-help"') == 18
    assert "First logical sender account index included in the workload." in report
    assert "EVM JSON-RPC URL used to query Ethereum-compatible blocks" in report
    assert "Highest transaction rate calculated over a rolling window" in report
    assert "Highest rolling average of committed transactions per second" in report
    assert "showFieldTooltip" in report


def test_generate_report_adds_fallback_tooltip_for_custom_config_fields(tmp_path):
    config_path = tmp_path / "config.yaml"
    stats_path = tmp_path / "stats.log"
    output_path = tmp_path / "report.html"
    config_path.write_text("custom_limit: 42\n")
    stats_path.write_text("")

    generate_report(
        config_path,
        stats_path,
        output_path,
        datetime(2026, 7, 19, 12, 0, tzinfo=timezone.utc),
    )

    report = output_path.read_text()
    assert "Configured benchmark value for custom limit." in report


def test_generate_report_sizes_y_axis_for_large_tick_labels(tmp_path):
    config_path = tmp_path / "config.yaml"
    stats_path = tmp_path / "stats.log"
    output_path = tmp_path / "report.html"
    config_path.write_text("chain_id: 777\n")
    stats_path.write_text(
        "block 11 txs=1 gas=363000000 " "2026-07-19T04:00:00+00:00 20ms tps=50.00\n"
    )

    generate_report(
        config_path,
        stats_path,
        output_path,
        datetime(2026, 7, 19, 12, 0, tzinfo=timezone.utc),
    )

    report = output_path.read_text()
    assert "ctx.measureText(label).width" in report
    assert "Math.ceil(maxTickWidth)+36" in report
