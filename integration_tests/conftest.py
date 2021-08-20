import re
from pathlib import Path

import pytest

from .network import setup_cronos, setup_geth


def pytest_configure(config):
    config.addinivalue_line("markers", "slow: marks tests as slow")
    config.addinivalue_line("markers", "gravity: gravity bridge test cases")


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
def suspend_capture(pytestconfig):
    """
    used to pause in testing

    Example:
    ```
    def test_simple(suspend_capture):
        with suspend_capture:
            # read user input
            print(input())
    """

    class SuspendGuard:
        def __init__(self):
            self.capmanager = pytestconfig.pluginmanager.getplugin("capturemanager")

        def __enter__(self):
            self.capmanager.suspend_global_capture(in_=True)

        def __exit__(self, _1, _2, _3):
            self.capmanager.resume_global_capture()

    yield SuspendGuard()


@pytest.fixture(scope="session", params=["cronos", "geth"])
def cluster(request, tmp_path_factory):
    provider = request.param
    path = tmp_path_factory.mktemp(provider)
    if provider == "cronos":
        yield from setup_cronos(path, 26650)
    elif provider == "geth":
        yield from setup_geth(path, 8545)
    else:
        raise NotImplementedError
