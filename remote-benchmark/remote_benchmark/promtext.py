"""Generic Prometheus text-exposition-format parsing helpers.

No CometBFT/Cosmos-specific knowledge lives here — see cometbft_metrics.py for
the consensus/block-stm scrapers built on top of these primitives.
"""


def fetch_prometheus_text(telemetry_url):
    """Fetch raw Prometheus text from the /metrics endpoint.

    Returns the response text, or empty string if unavailable/not configured.
    """
    if not telemetry_url:
        return ""

    import requests as _requests

    try:
        resp = _requests.get(f"{telemetry_url}/metrics", timeout=5)
        resp.raise_for_status()
        return resp.text
    except Exception:
        return ""


def parse_histogram_sum_count(lines, metric_name, label_filter=None):
    """Return (sum, count) from a Prometheus histogram's _sum/_count lines.

    Samples are accumulated, not overwritten: a histogram split across several
    label sets (e.g. one per ABCI method) emits one _sum/_count pair per set,
    and the aggregate over all matching lines is what the callers want.

    Returns (sum_or_None, count). Sum is in the metric's native unit.
    """
    total = None
    count = 0
    for line in lines:
        if line.startswith("#"):
            continue
        if label_filter and label_filter not in line:
            continue
        if f"{metric_name}_sum" in line:
            total = (total or 0.0) + float(line.split()[-1])
        elif f"{metric_name}_count" in line:
            count += int(float(line.split()[-1]))
    return total, count


def parse_label_block(line, metric_name):
    """Parse `{k="v",...} value` following a metric name into (labels, value).

    Scans character by character because a quoted label value may legally
    contain `,` and `}`, which splitting on those characters mis-parses.
    Returns None for a line whose label list is never closed.
    """
    rest = line[len(metric_name) + 1 :]  # skip past the opening '{'
    items = []
    current = []
    in_quotes = False
    escaped = False
    for idx, char in enumerate(rest):
        if escaped:
            current.append(char)
            escaped = False
        elif in_quotes and char == "\\":
            escaped = True
        elif char == '"':
            in_quotes = not in_quotes
        elif char == "," and not in_quotes:
            items.append("".join(current))
            current = []
        elif char == "}" and not in_quotes:
            items.append("".join(current))
            tail = rest[idx + 1 :].split()
            if not tail:
                return None
            labels = dict(item.split("=", 1) for item in items if item)
            return labels, float(tail[-1])
        else:
            current.append(char)
    return None


def parse_labeled_metric(lines, metric_name):
    """Parse every sample of a labeled or unlabeled Prometheus counter/gauge.

    Returns a list of (labels_dict, value) — one entry per distinct label set.
    Matches `metric_name` exactly (either `name{...} value` or `name value`),
    so metric names that share a prefix (e.g. `..._rounds` vs
    `..._round_increment_total`) don't cross-match.
    """
    results = []
    labeled_prefix = metric_name + "{"
    bare_prefix = metric_name + " "
    for line in lines:
        if line.startswith("#"):
            continue
        if line.startswith(labeled_prefix):
            parsed = parse_label_block(line, metric_name)
            if parsed is not None:
                results.append(parsed)
        elif line.startswith(bare_prefix):
            results.append(({}, float(line.split()[-1])))
    return results


def labeled_metric_by(lines, metric_name, label_key):
    """{label_value: value} for a labeled metric, keyed by one of its labels."""
    return {
        labels[label_key]: value
        for labels, value in parse_labeled_metric(lines, metric_name)
        if label_key in labels
    }
