import os
import re
import signal
import subprocess
import time
from pathlib import Path

import pytest
import web3
from web3.middleware import geth_poa_middleware

from .utils import cluster_fixture, wait_for_ipc, wait_for_port


def pytest_configure(config):
    config.addinivalue_line("markers", "slow: marks tests as slow")


def pytest_addoption(parser):
    parser.addoption(
        "--supervisord-quiet",
        dest="supervisord-quiet",
        action="store_true",
        default=False,
        help="redirect supervisord's stdout to file",
    )


@pytest.fixture(scope="session")
def worker_index(worker_id):
    match = re.search(r"\d+", worker_id)
    return int(match[0]) if match is not None else 0


@pytest.fixture(scope="session")
def cluster(worker_index, pytestconfig, tmp_path_factory):
    "default cluster fixture"
    yield from cluster_fixture(
        Path(__file__).parent / "configs/default.yaml",
        worker_index,
        tmp_path_factory.mktemp("data"),
        quiet=pytestconfig.getoption("supervisord-quiet"),
    )


@pytest.fixture(scope="session")
def suspend_capture(pytestconfig):
    "used for pause in testing"

    class SuspendGuard:
        def __init__(self):
            self.capmanager = pytestconfig.pluginmanager.getplugin("capturemanager")

        def __enter__(self):
            self.capmanager.suspend_global_capture(in_=True)

        def __exit__(self, _1, _2, _3):
            self.capmanager.resume_global_capture()

    yield SuspendGuard()


def setup_cronos(path):
    proc = subprocess.Popen(
        ["start-cronos", path], preexec_fn=os.setsid, stdout=subprocess.PIPE
    )
    try:
        wait_for_port(1317)
        yield web3.Web3(web3.providers.HTTPProvider("http://localhost:1317"))
    finally:
        os.killpg(os.getpgid(proc.pid), signal.SIGTERM)
        # proc.terminate()
        proc.wait()


def setup_geth(path):
    proc = subprocess.Popen(
        ["start-geth", path], preexec_fn=os.setsid, stdout=subprocess.PIPE
    )
    try:
        ipc_path = path / "geth.ipc"
        wait_for_ipc(ipc_path)
        w3 = web3.Web3(web3.providers.IPCProvider(ipc_path))
        w3.middleware_onion.inject(geth_poa_middleware, layer=0)
        time.sleep(1)
        yield w3
    finally:
        os.killpg(os.getpgid(proc.pid), signal.SIGTERM)
        # proc.terminate()
        proc.wait()


@pytest.fixture(scope="session", params=["cronos", "geth"])
def w3(request, tmp_path_factory):
    provider = request.param
    path = tmp_path_factory.mktemp(provider)
    if provider == "cronos":
        yield from setup_cronos(path)
    elif provider == "geth":
        yield from setup_geth(path)
    else:
        raise NotImplementedError
