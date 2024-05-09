import concurrent.futures
import ipaddress
from datetime import datetime

from .params import RunParams
from .sync import SyncService

SYNC_SERVICE_URL = "ws://localhost:5050"
TEST_PARAMS = RunParams(
    test_case="entrypoint",
    test_group_id="single",
    test_group_instance_count=2,
    test_instance_count=2,
    test_instance_params=("latency=0|timeout=21m|bandwidth=420Mib|chain_id=testground"),
    test_outputs_path="/outputs",
    test_plan="benchmark",
    test_run="cp9va5nae0pksdti05vg",
    test_start_time=datetime.fromisoformat("2024-05-27T10:52:08+08:00"),
    test_subnet=ipaddress.IPv4Network("16.20.0.0/16"),
    test_sidecar=True,
    test_temp_path="/temp",
)


def test_barrier():
    sync = SyncService(TEST_PARAMS, SYNC_SERVICE_URL)
    state = "test1"
    target = 10
    for i in range(target):
        sync.signal_entry(state)
    sync.barrier(state, target)
    sync.close()


def test_signal_and_wait():
    sync = SyncService(TEST_PARAMS, SYNC_SERVICE_URL)
    state = "test_signal_and_wait"
    target = 10

    def do_test():
        sync.signal_and_wait(state, target)

    with concurrent.futures.ThreadPoolExecutor(max_workers=target) as executor:
        futs = [executor.submit(do_test) for i in range(target)]
        for fut in futs:
            fut.result(1)

    sync.close()
