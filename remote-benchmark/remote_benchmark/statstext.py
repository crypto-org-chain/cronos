"""Plain text parsing of a dump_block_stats/dump_eth_block_stats report.

No presentation concerns here (see report.py for HTML rendering) — this is
the data-layer counterpart that results.py depends on when building a run
record, so it must not carry an import-time dependency on anything that
renders output.
"""

import re

BLOCK_RE = re.compile(r"^block (?P<height>\d+) txs=(?P<txs>\d+)(?P<rest>.*)$")
METRIC_RE = re.compile(r"^(?P<name>[a-z][a-z0-9_]*) (?P<value>.+)$")
TIMESTAMP_RE = re.compile(r"\b(?P<timestamp>\d{4}-\d{2}-\d{2}[T ]\S+)")


def parse_stats(text: str) -> tuple[list[dict], dict[str, str]]:
    blocks = []
    metrics = {}
    for line in text.splitlines():
        block_match = BLOCK_RE.match(line)
        if block_match:
            rest = block_match.group("rest")
            tps_match = re.search(r"\btps=([0-9.]+)", rest)
            gas_match = re.search(r"\bgas=(\d+)", rest)
            timestamp_match = TIMESTAMP_RE.search(rest)
            blocks.append(
                {
                    "height": int(block_match.group("height")),
                    "transactions": int(block_match.group("txs")),
                    "gas_consumed": int(gas_match.group(1)) if gas_match else 0,
                    "tps": float(tps_match.group(1)) if tps_match else 0,
                    "timestamp": (
                        timestamp_match.group("timestamp") if timestamp_match else None
                    ),
                }
            )
            continue

        metric_match = METRIC_RE.match(line)
        if metric_match:
            metrics[metric_match.group("name")] = metric_match.group("value")
    return blocks, metrics
