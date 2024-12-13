import concurrent.futures
from concurrent.futures import ThreadPoolExecutor, as_completed

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
    params = []
    for _ in range(batch):
        params.append(param)
    with ThreadPoolExecutor(concurrent) as executor:
        tasks = [executor.submit(call, url, params) for _ in range(0, concurrent)]
        results = [future.result() for future in as_completed(tasks)]
    assert len(results) == concurrent


def call_trace(url):
    res = requests.get(f"{url}/debug/pprof/trace?seconds=3")
    if res.status_code == 200:
        with open("trace.out", "wb") as file:
            file.write(res.content)
        print("saved trace.out")
    else:
        print(f"failed to retrieve data: {res.status_code}")


def test_cosmovisor_upgrade(custom_cronos):
    c = custom_cronos
    do_upgrade(c, "v1.4", c.cosmos_cli().block_height() + 15)
    res = deploy_contract(c.w3, CONTRACTS["CheckpointOracle"])
    with ThreadPoolExecutor() as exec:
        futures = [
            exec.submit(call_trace, c.pprof_endpoint()),
            exec.submit(call_check, c.w3_http_endpoint(), res.address, 100),
        ]
        concurrent.futures.wait(futures)
