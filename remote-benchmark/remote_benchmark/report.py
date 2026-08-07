import argparse
import html
import json
import re
from datetime import datetime, timezone
from importlib import resources
from pathlib import Path

import yaml

from .htmlutil import display_value as _display
from .htmlutil import field_label as _field_label
from .htmlutil import flatten as _flatten
from .statstext import parse_stats

_ASSETS = resources.files("remote_benchmark.assets")
_REPORT_CSS = _ASSETS.joinpath("report.css").read_text()
_REPORT_JS = _ASSETS.joinpath("report.js").read_text()

PARAMETER_TOOLTIPS = {
    "benchmark.validators": "Number of validator nodes participating in the benchmark network.",
    "benchmark.testcase": "Workload scenario executed by this benchmark run.",
    "benchmark.start_account": "First logical sender account index included in the workload.",
    "benchmark.end_account": "Last logical sender account index included in the workload, inclusive.",
    "benchmark.generated_at": "Local date and time when this report was generated.",
    "mode": "Transaction wire format: Cosmos wraps EVM messages in Cosmos transactions; eth sends raw Ethereum transactions.",
    "chain_id": "EVM chain ID included when signing each generated transaction.",
    "evm_denom": "Base denomination used to express EVM transaction fees on the chain.",
    "gas_price": "Gas price, in the smallest denomination, used for generated EVM transactions.",
    "global_seq": "Global account-generation sequence used to derive deterministic benchmark sender keys.",
    "tx_type": "Type of EVM operation generated for the workload, such as a native or ERC20 transfer.",
    "msg_version": "Version of the Cosmos MsgEthereumTx encoding used for wrapped EVM transactions.",
    "num_accounts": "Number of logical sender accounts configured for the workload.",
    "num_txs": "Number of EVM transactions generated for each logical sender account.",
    "sender_strategy": "Whether senders are reused for sequential transactions or each transaction gets a unique sender.",
    "batch_size": "Number of EVM messages packed into each Cosmos transaction; 1 means no message batching.",
    "send_batch_size": "Maximum number of signed transactions broadcast concurrently in one sending batch.",
    "send_interval": "Pause, in seconds, between consecutive sending batches.",
    "telemetry": "Prometheus telemetry endpoint used to collect consensus and BlockSTM statistics.",
}

RESULT_TOOLTIPS = {
    "Peak TPS": "Highest transaction rate calculated over a rolling window of up to 10 blocks, excluding detected stalls.",
    "Overall TPS": "Committed transactions divided by measured elapsed time after detected stall blocks and their time are excluded.",
    "Total transactions": "Total number of inner EVM transactions committed during the measured benchmark window.",
    "Committed Cosmos txs": "Number of generated Cosmos transaction envelopes that committed, shown against the number sent.",
    "Peak 1s TPS": "Largest number of transactions committed in any one wall-clock second.",
    "Peak 5s avg TPS": "Highest rolling average of committed transactions per second across a five-second window.",
    "Peak gas / second": "Largest total amount of EVM gas consumed in any one wall-clock second.",
}


def _parameter_tooltip(name: str) -> str:
    endpoint_match = re.fullmatch(r"endpoints\[(\d+)]\.(name|rpc|json_rpc)", name)
    if endpoint_match:
        index, field = endpoint_match.groups()
        descriptions = {
            "name": "Human-readable name of this benchmark target node.",
            "rpc": "CometBFT RPC URL used to broadcast Cosmos transactions and query blocks.",
            "json_rpc": "EVM JSON-RPC URL used to query Ethereum-compatible blocks and transactions.",
        }
        return f"Endpoint {int(index) + 1}: {descriptions[field]}"
    if name in PARAMETER_TOOLTIPS:
        return PARAMETER_TOOLTIPS[name]
    readable_name = name.replace("_", " ")
    return f"Configured benchmark value for {readable_name}."


def bucket_by_second(blocks: list[dict]) -> list[dict]:
    """Aggregate the active transaction window into wall-clock seconds."""
    timestamped = []
    for block in blocks:
        if not block["timestamp"]:
            continue
        timestamped.append((datetime.fromisoformat(block["timestamp"]), block))

    active = [item for item in timestamped if item[1]["transactions"] > 0]
    if not active:
        return []

    first_second = int(active[0][0].timestamp())
    last_second = int(active[-1][0].timestamp())
    totals = {
        second: {"transactions": 0, "gas_consumed": 0}
        for second in range(first_second, last_second + 1)
    }
    for timestamp, block in timestamped:
        second = int(timestamp.timestamp())
        if second in totals:
            totals[second]["transactions"] += block["transactions"]
            totals[second]["gas_consumed"] += block["gas_consumed"]

    result = []
    transaction_counts = []
    for second, values in totals.items():
        transaction_counts.append(values["transactions"])
        rolling_window = transaction_counts[-5:]
        result.append(
            {
                "elapsed_second": second - first_second,
                "timestamp": datetime.fromtimestamp(second, timezone.utc).isoformat(),
                **values,
                "rolling_tps_5s": sum(rolling_window) / len(rolling_window),
            }
        )
    return result


def _build_view_model(
    config: dict,
    stats_text: str,
    generated_at: datetime,
    validators: int | None,
    testcase: str | None,
    start_account: int | None,
    end_account: int | None,
) -> dict:
    """Assemble the data render_report needs, independent of HTML formatting."""
    blocks, metrics = parse_stats(stats_text)
    second_buckets = bucket_by_second(blocks)
    params = {
        "benchmark.validators": validators,
        "benchmark.testcase": testcase,
        "benchmark.start_account": start_account,
        "benchmark.end_account": end_account,
        "benchmark.generated_at": generated_at.astimezone().isoformat(
            timespec="seconds"
        ),
    }
    params.update(dict(_flatten(config)))

    featured = [
        ("Peak TPS", metrics.get("peak_tps", "N/A")),
        ("Overall TPS", metrics.get("overall_tps", "N/A")),
        ("Total transactions", metrics.get("total_txs", "N/A")),
        ("Committed Cosmos txs", metrics.get("committed_cosmos_txs", "N/A")),
    ]
    if second_buckets:
        featured.extend(
            [
                (
                    "Peak 1s TPS",
                    f"{max(bucket['transactions'] for bucket in second_buckets):,}",
                ),
                (
                    "Peak 5s avg TPS",
                    f"{max(bucket['rolling_tps_5s'] for bucket in second_buckets):,.1f}",
                ),
                (
                    "Peak gas / second",
                    f"{max(bucket['gas_consumed'] for bucket in second_buckets):,}",
                ),
            ]
        )

    title_bits = [str(validators) + " validator" + ("s" if validators != 1 else "")]
    if testcase:
        title_bits.append(testcase)
    title = " / ".join(title_bits) if validators else "Benchmark"

    return {
        "params": params,
        "featured": featured,
        "chart_data": json.dumps(blocks, separators=(",", ":")).replace(
            "<", "\\u003c"
        ),
        "second_chart_data": json.dumps(second_buckets, separators=(",", ":")).replace(
            "<", "\\u003c"
        ),
        "title": title,
        "generated_label": generated_at.astimezone().strftime("%Y-%m-%d %H:%M:%S %Z"),
    }


def render_report(
    config: dict,
    stats_text: str,
    generated_at: datetime,
    validators: int | None = None,
    testcase: str | None = None,
    start_account: int | None = None,
    end_account: int | None = None,
) -> str:
    view = _build_view_model(
        config, stats_text, generated_at, validators, testcase, start_account, end_account
    )

    param_rows = "\n".join(
        f"<tr><th>{_field_label(name, _parameter_tooltip(name))}</th>"
        f"<td>{html.escape(_display(value))}</td></tr>"
        for name, value in view["params"].items()
        if value is not None
    )
    metric_cards = "\n".join(
        f'<div class="metric"><span>{_field_label(label, RESULT_TOOLTIPS[label])}</span>'
        f"<strong>{html.escape(value)}</strong></div>"
        for label, value in view["featured"]
    )
    title = view["title"]

    return f"""<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{html.escape(title)} benchmark report</title>
  <style>
{_REPORT_CSS}
  </style>
</head>
<body>
<main>
  <h1>{html.escape(title)} benchmark report</h1>
  <p class="timestamp">Generated {html.escape(view["generated_label"])}</p>
  <h2>Parameters</h2>
  <div class="params"><table><tbody>{param_rows}</tbody></table></div>
  <h2>Results</h2>
  <div class="metrics">{metric_cards}</div>
  <div class="field-tooltip" id="fieldTooltip" role="tooltip"></div>
  <h2>Transactions by block</h2>
  <div class="chart-wrap" id="chartWrap">
    <canvas id="chart" role="img" aria-label="Transaction count for each block height"></canvas>
    <div class="tooltip" id="tooltip"></div>
  </div>
  <h2>Gas consumed by block</h2>
  <div class="chart-wrap" id="gasChartWrap">
    <canvas id="gasChart" role="img" aria-label="Gas consumed for each block height"></canvas>
    <div class="tooltip" id="gasTooltip"></div>
  </div>
  <h2>Transactions per second</h2>
  <div class="chart-wrap" id="secondChartWrap">
    <canvas id="secondChart" role="img" aria-label="Transactions committed per elapsed second"></canvas>
    <div class="tooltip" id="secondTooltip"></div>
  </div>
  <h2>Gas consumed per second</h2>
  <div class="chart-wrap" id="secondGasChartWrap">
    <canvas id="secondGasChart" role="img" aria-label="Gas consumed per elapsed second"></canvas>
    <div class="tooltip" id="secondGasTooltip"></div>
  </div>
</main>
<script>
const data={view["chart_data"]};
const secondData={view["second_chart_data"]};
{_REPORT_JS}
</script>
</body>
</html>
"""


def generate_report(
    config_path: Path,
    stats_path: Path,
    output_path: Path,
    generated_at: datetime,
    validators: int | None = None,
    testcase: str | None = None,
    start_account: int | None = None,
    end_account: int | None = None,
) -> None:
    config = yaml.safe_load(config_path.read_text())
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(
        render_report(
            config,
            stats_path.read_text(),
            generated_at,
            validators=validators,
            testcase=testcase,
            start_account=start_account,
            end_account=end_account,
        )
    )


def main() -> None:
    parser = argparse.ArgumentParser(description="Generate a benchmark HTML report")
    parser.add_argument("--config", type=Path, required=True)
    parser.add_argument("--stats", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--timestamp", required=True)
    parser.add_argument("--validators", type=int)
    parser.add_argument("--testcase")
    parser.add_argument("--start-account", type=int)
    parser.add_argument("--end-account", type=int)
    args = parser.parse_args()
    generated_at = datetime.fromisoformat(args.timestamp)
    generate_report(
        args.config,
        args.stats,
        args.output,
        generated_at,
        validators=args.validators,
        testcase=args.testcase,
        start_account=args.start_account,
        end_account=args.end_account,
    )


if __name__ == "__main__":
    main()
