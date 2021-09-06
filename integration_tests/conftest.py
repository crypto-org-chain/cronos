import pytest

from .network import setup_cronos, setup_geth


def pytest_configure(config):
    config.addinivalue_line("markers", "slow: marks tests as slow")
    config.addinivalue_line("markers", "gravity: gravity bridge test cases")


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
    ```
    """

    class SuspendGuard:
        def __init__(self):
            self.capmanager = pytestconfig.pluginmanager.getplugin("capturemanager")

        def __enter__(self):
            self.capmanager.suspend_global_capture(in_=True)

        def __exit__(self, _1, _2, _3):
            self.capmanager.resume_global_capture()

    yield SuspendGuard()


@pytest.fixture(scope="session")
def cronos(tmp_path_factory):
    path = tmp_path_factory.mktemp("cronos")
    yield from setup_cronos(path, 26650)


@pytest.fixture(scope="session")
def geth(tmp_path_factory):
    path = tmp_path_factory.mktemp("geth")
    yield from setup_geth(path, 8545)


@pytest.fixture(scope="session", params=["cronos", "geth"])
def cluster(request, cronos, geth):
    """
    run on both cronos and geth
    """
    provider = request.param
    if provider == "cronos":
        yield cronos
    elif provider == "geth":
        yield geth
    else:
        raise NotImplementedError
