"""Resource telemetry: Go runtime gauges and node_exporter disk/network I/O.

Go runtime metrics (goroutines, heap, RSS) come from the same CometBFT
/metrics endpoint already scraped for consensus telemetry — client_golang
registers them by default. Disk and network counters need a separate
node_exporter target, since CometBFT doesn't expose host-level I/O.
"""

from .stats import _fetch_prometheus, _parse_labeled_metric, _sum_labeled_metric

fetch_node_exporter = _fetch_prometheus


def scrape_go_runtime(prom_text):
    """Instantaneous Go runtime + process gauges (RSS, heap, goroutines)."""
    lines = prom_text.splitlines()

    def _gauge(name):
        values = _parse_labeled_metric(lines, name)
        return values[0][1] if values else None

    return {
        "goroutines": _gauge("go_goroutines"),
        "heap_alloc_bytes": _gauge("go_memstats_heap_alloc_bytes"),
        "heap_inuse_bytes": _gauge("go_memstats_heap_inuse_bytes"),
        "alloc_bytes": _gauge("go_memstats_alloc_bytes"),
        "sys_bytes": _gauge("go_memstats_sys_bytes"),
        "rss_bytes": _gauge("process_resident_memory_bytes"),
    }


_DISK_NET_COUNTERS = [
    ("disk_read_bytes", "node_disk_read_bytes_total"),
    ("disk_written_bytes", "node_disk_written_bytes_total"),
    ("disk_reads_completed", "node_disk_reads_completed_total"),
    ("disk_writes_completed", "node_disk_writes_completed_total"),
]


def scrape_disk_net_raw(node_exporter_text):
    """Snapshot raw cumulative disk/network counters (see scrape_disk_net for
    the baseline-relative view). Network counters exclude the loopback
    device, which otherwise dwarfs real traffic on a single-host devnet."""
    lines = node_exporter_text.splitlines()
    raw = {key: _sum_labeled_metric(lines, metric) for key, metric in _DISK_NET_COUNTERS}
    for key, metric in [
        ("network_receive_bytes", "node_network_receive_bytes_total"),
        ("network_transmit_bytes", "node_network_transmit_bytes_total"),
    ]:
        raw[key] = sum(
            value for labels, value in _parse_labeled_metric(lines, metric) if labels.get("device") != "lo"
        )
    return raw


def scrape_disk_net(node_exporter_text, baseline=None):
    """Disk and network I/O over the load period, as a delta against a
    baseline snapshot taken from scrape_disk_net_raw at load start."""
    raw = scrape_disk_net_raw(node_exporter_text)
    if baseline:
        return {key: raw[key] - baseline.get(key, 0) for key in raw}
    return raw
