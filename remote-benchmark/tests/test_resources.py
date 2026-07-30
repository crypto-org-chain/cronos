from remote_benchmark.resources import scrape_disk_net, scrape_disk_net_raw, scrape_go_runtime

GO_RUNTIME_TEXT = """\
# HELP go_goroutines Number of goroutines.
# TYPE go_goroutines gauge
go_goroutines 42
# HELP go_memstats_heap_alloc_bytes Heap bytes allocated and in use.
# TYPE go_memstats_heap_alloc_bytes gauge
go_memstats_heap_alloc_bytes 1048576
# HELP go_memstats_heap_inuse_bytes Heap bytes in use.
# TYPE go_memstats_heap_inuse_bytes gauge
go_memstats_heap_inuse_bytes 2097152
# HELP go_memstats_alloc_bytes Bytes allocated and in use.
# TYPE go_memstats_alloc_bytes gauge
go_memstats_alloc_bytes 3145728
# HELP go_memstats_sys_bytes Bytes obtained from system.
# TYPE go_memstats_sys_bytes gauge
go_memstats_sys_bytes 4194304
# HELP process_resident_memory_bytes Resident memory size in bytes.
# TYPE process_resident_memory_bytes gauge
process_resident_memory_bytes 5242880
"""

NODE_EXPORTER_TEXT = """\
# HELP node_disk_read_bytes_total Bytes read from disk.
# TYPE node_disk_read_bytes_total counter
node_disk_read_bytes_total{device="sda"} 1000
node_disk_read_bytes_total{device="sdb"} 500
# HELP node_disk_written_bytes_total Bytes written to disk.
# TYPE node_disk_written_bytes_total counter
node_disk_written_bytes_total{device="sda"} 2000
# HELP node_disk_reads_completed_total Disk reads completed.
# TYPE node_disk_reads_completed_total counter
node_disk_reads_completed_total{device="sda"} 10
# HELP node_disk_writes_completed_total Disk writes completed.
# TYPE node_disk_writes_completed_total counter
node_disk_writes_completed_total{device="sda"} 20
# HELP node_network_receive_bytes_total Network bytes received.
# TYPE node_network_receive_bytes_total counter
node_network_receive_bytes_total{device="eth0"} 9000
node_network_receive_bytes_total{device="lo"} 123456789
# HELP node_network_transmit_bytes_total Network bytes transmitted.
# TYPE node_network_transmit_bytes_total counter
node_network_transmit_bytes_total{device="eth0"} 4000
node_network_transmit_bytes_total{device="lo"} 123456789
"""


def test_scrape_go_runtime_reads_gauges():
    go = scrape_go_runtime(GO_RUNTIME_TEXT)

    assert go == {
        "goroutines": 42.0,
        "heap_alloc_bytes": 1048576.0,
        "heap_inuse_bytes": 2097152.0,
        "alloc_bytes": 3145728.0,
        "sys_bytes": 4194304.0,
        "rss_bytes": 5242880.0,
    }


def test_scrape_go_runtime_missing_metrics_are_none():
    assert scrape_go_runtime("") == {
        "goroutines": None,
        "heap_alloc_bytes": None,
        "heap_inuse_bytes": None,
        "alloc_bytes": None,
        "sys_bytes": None,
        "rss_bytes": None,
    }


def test_scrape_disk_net_raw_sums_devices_and_excludes_loopback():
    raw = scrape_disk_net_raw(NODE_EXPORTER_TEXT)

    assert raw["disk_read_bytes"] == 1500
    assert raw["disk_written_bytes"] == 2000
    assert raw["disk_reads_completed"] == 10
    assert raw["disk_writes_completed"] == 20
    assert raw["network_receive_bytes"] == 9000
    assert raw["network_transmit_bytes"] == 4000


def test_scrape_disk_net_with_baseline_reports_deltas():
    baseline = scrape_disk_net_raw(NODE_EXPORTER_TEXT)
    grown_text = NODE_EXPORTER_TEXT.replace('device="sda"} 1000', 'device="sda"} 1600')

    delta = scrape_disk_net(grown_text, baseline=baseline)

    assert delta["disk_read_bytes"] == 600
    assert delta["disk_written_bytes"] == 0
