"""Compare two `bench` run records (single-run or `--repeat` aggregate).

Produces a per-metric delta table, an explicit "spreads overlap, not
significant" flag when both sides carry a stdev, and a diff of the two run
configurations.
"""

import html
import math
from pathlib import Path

import ujson

from .htmlutil import display_value, field_label, flatten

# Metrics carrying non-scalar data (lists/sets used for the HTML report's
# charts) aren't meaningful in a delta table.
_EXCLUDED_METRIC_KEYS = {
    "gas_utilizations",
    "tx_gas_list",
    "steady_block_times",
    "stall_height_offsets",
}


def load_record(path):
    return ujson.loads(Path(path).read_text())


def extract_metrics(record):
    """Normalize a run record's numeric metrics to {name: {value, stdev, n}}.

    Single-run records carry a flat `summary` dict (n=1, no stdev).
    `--repeat` aggregate records carry `aggregate[name] = {median, stdev, n}`.
    """
    if record.get("run_kind") == "bench-aggregate":
        return {
            name: {"value": entry["median"], "stdev": entry["stdev"], "n": entry["n"]}
            for name, entry in (record.get("aggregate") or {}).items()
        }

    summary = record.get("summary") or {}
    return {
        name: {"value": value, "stdev": None, "n": 1}
        for name, value in summary.items()
        if name not in _EXCLUDED_METRIC_KEYS
        and isinstance(value, (int, float))
        and not isinstance(value, bool)
    }


def _overlaps(a, b):
    """True if a's and b's [value-stdev, value+stdev] ranges overlap."""
    a_low, a_high = a["value"] - a["stdev"], a["value"] + a["stdev"]
    b_low, b_high = b["value"] - b["stdev"], b["value"] + b["stdev"]
    return a_low <= b_high and b_low <= a_high


def compare_metrics(metrics_a, metrics_b):
    """Return one row per metric present on both sides, sorted by name."""
    rows = []
    for name in sorted(set(metrics_a) & set(metrics_b)):
        a, b = metrics_a[name], metrics_b[name]
        delta = b["value"] - a["value"]
        if a["value"]:
            pct_change = delta / a["value"] * 100
        elif b["value"]:
            # Zero baseline with a nonzero comparison value is a real change
            # (e.g. failed_txs 0 -> 100) - report it, not "n/a".
            pct_change = math.inf if delta > 0 else -math.inf
        else:
            pct_change = 0.0

        if a["stdev"] is not None and b["stdev"] is not None and a["n"] > 1 and b["n"] > 1:
            significant = not _overlaps(a, b)
        else:
            significant = None  # not enough samples to judge significance

        rows.append(
            {
                "metric": name,
                "a": a["value"],
                "b": b["value"],
                "delta": delta,
                "pct_change": pct_change,
                "significant": significant,
            }
        )
    return rows


def diff_config(config_a, config_b):
    """Flatten both configs and return keys whose values differ."""
    flat_a = dict(flatten(config_a))
    flat_b = dict(flatten(config_b))
    return [
        {"key": key, "a": flat_a.get(key), "b": flat_b.get(key)}
        for key in sorted(set(flat_a) | set(flat_b))
        if flat_a.get(key) != flat_b.get(key)
    ]


def build_comparison(record_a, record_b, label_a, label_b):
    metric_rows = compare_metrics(extract_metrics(record_a), extract_metrics(record_b))
    config_rows = diff_config(record_a.get("config") or {}, record_b.get("config") or {})
    return {
        "label_a": label_a,
        "label_b": label_b,
        "metrics": metric_rows,
        "config_diff": config_rows,
    }


def _fmt_num(value):
    if isinstance(value, float):
        return f"{value:,.4g}"
    return f"{value:,}"


def _significance_label(significant):
    if significant is None:
        return "n/a (single run)"
    return "significant" if significant else "not significant (spreads overlap)"


def render_comparison_text(comparison):
    lines = [f"comparing {comparison['label_a']!r} vs {comparison['label_b']!r}", ""]
    for row in comparison["metrics"]:
        pct = f"{row['pct_change']:+.1f}%" if row["pct_change"] is not None else "n/a"
        lines.append(
            f"{row['metric']}: {_fmt_num(row['a'])} -> {_fmt_num(row['b'])} "
            f"({pct}, {_significance_label(row['significant'])})"
        )
    if comparison["config_diff"]:
        lines.append("")
        lines.append("config differences:")
        for row in comparison["config_diff"]:
            lines.append(f"  {row['key']}: {row['a']!r} -> {row['b']!r}")
    return "\n".join(lines)


def render_comparison_html(comparison):
    def _pct_cell(pct_change):
        return f"{pct_change:+.1f}%" if pct_change is not None else "n/a"

    metric_rows = "\n".join(
        f"<tr><td>{html.escape(row['metric'])}</td>"
        f"<td>{html.escape(_fmt_num(row['a']))}</td>"
        f"<td>{html.escape(_fmt_num(row['b']))}</td>"
        f"<td>{html.escape(_pct_cell(row['pct_change']))}</td>"
        f"<td>{html.escape(_significance_label(row['significant']))}</td></tr>"
        for row in comparison["metrics"]
    )
    config_rows = "\n".join(
        f"<tr><th>{field_label(row['key'], 'Configuration value that differs between the two runs.')}</th>"
        f"<td>{html.escape(display_value(row['a']))}</td><td>{html.escape(display_value(row['b']))}</td></tr>"
        for row in comparison["config_diff"]
    )
    label_a, label_b = html.escape(comparison["label_a"]), html.escape(comparison["label_b"])

    return f"""<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Benchmark comparison: {label_a} vs {label_b}</title>
  <style>
    :root {{ color-scheme: light; --ink:#182026; --muted:#66717a; --line:#d8dee3;
      --surface:#fff; --page:#f3f5f6; --accent:#087e8b; }}
    * {{ box-sizing:border-box; }}
    body {{ margin:0; background:var(--page); color:var(--ink); font:14px/1.5 system-ui,sans-serif; }}
    main {{ width:min(1180px, calc(100% - 32px)); margin:0 auto; padding:32px 0 56px; }}
    h1 {{ margin:0; font-size:26px; }}
    h2 {{ margin:32px 0 12px; font-size:18px; }}
    table {{ width:100%; border-collapse:collapse; background:var(--surface); border:1px solid var(--line); }}
    th,td {{ padding:9px 12px; border-bottom:1px solid var(--line); text-align:left; vertical-align:top; }}
    th {{ color:#344047; font-weight:600; background:#fafbfb; }}
    tr:last-child th,tr:last-child td {{ border-bottom:0; }}
  </style>
</head>
<body>
<main>
  <h1>Benchmark comparison</h1>
  <p>{label_a} vs {label_b}</p>
  <h2>Metrics</h2>
  <table><thead><tr><th>Metric</th><th>{label_a}</th><th>{label_b}</th>
    <th>% change</th><th>Significance</th></tr></thead>
    <tbody>{metric_rows}</tbody></table>
  <h2>Configuration differences</h2>
  <table><thead><tr><th>Key</th><th>{label_a}</th><th>{label_b}</th></tr></thead>
    <tbody>{config_rows or '<tr><td colspan="3">No configuration differences.</td></tr>'}</tbody></table>
</main>
</body>
</html>
"""


def write_comparison_html(comparison, output_path):
    path = Path(output_path)
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(render_comparison_html(comparison))
    return path
