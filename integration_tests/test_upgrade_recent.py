import subprocess
from concurrent.futures import ThreadPoolExecutor, as_completed
from pathlib import Path

import pytest
import requests

from .network import setup_upgrade_cronos
from .utils import CONTRACTS, deploy_contract, do_upgrade

pytestmark = pytest.mark.upgrade


@pytest.fixture(scope="module")
def custom_cronos(tmp_path_factory):
    port = 27100
    nix_name = "upgrade-test-package-recent"
    cfg_name = "cosmovisor_recent"
    yield from setup_upgrade_cronos(tmp_path_factory, port, nix_name, cfg_name)


def call(url, params):
    rsp = requests.post(url, json=params)
    assert rsp.status_code == 200
    return rsp.json()


def call_check(url, address, concurrent):
    batch = 10
    param = {
        "jsonrpc": "2.0",
        "method": "eth_call",
        "params": [
            {
                "data": "0x0be5b6ba",
                "to": address,
            },
            "latest",
        ],
        "id": 1,
    }
    params = [param for _ in range(batch)]
    with ThreadPoolExecutor(concurrent) as executor:
        tasks = [executor.submit(call, url, params) for _ in range(0, concurrent)]
        results = [future.result() for future in as_completed(tasks)]
    assert len(results) == concurrent


def call_trace(url, tmp_path_factory):
    res = requests.get(f"{url}/debug/pprof/trace?seconds=0.2")
    assert res.status_code == 200, res
    folder = tmp_path_factory.mktemp("trace")
    trace = Path(folder / "trace.out")
    syscall = Path(folder / "syscall.out")
    with open(trace, "wb") as file:
        file.write(res.content)
    cmd = f"go tool trace -pprof=syscall {trace} > {syscall}"
    subprocess.run(cmd, shell=True, check=True)
    cmd = f"go tool pprof -top {syscall} | head -n 10"
    res = subprocess.run(
        cmd, shell=True, check=True, capture_output=True, text=True
    ).stdout
    print(res)
    assert "rocksdb" in res and "create_iterator" not in res, res


def test_cosmovisor_upgrade(custom_cronos, tmp_path_factory):
    c = custom_cronos
    do_upgrade(c, "v1.4", c.cosmos_cli().block_height() + 15)
    res = deploy_contract(c.w3, CONTRACTS["CheckpointOracle"])
    with ThreadPoolExecutor() as exec:
        tasks = [
            exec.submit(call_trace, c.pprof_endpoint(), tmp_path_factory),
            exec.submit(call_check, c.w3_http_endpoint(), res.address, 100),
        ]
        [future.result() for future in as_completed(tasks)]
