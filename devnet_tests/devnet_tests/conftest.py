import pytest

from .devnet import load_devnet


def pytest_configure(config):
    config.addinivalue_line(
        "markers", "rpc_diff: test replays a request corpus across >=2 nodes"
    )


def pytest_addoption(parser):
    parser.addoption(
        "--devnet-config",
        default=None,
        help="path to a YAML config listing the devnet node(s) to test against",
    )


@pytest.fixture(scope="session")
def devnet(request):
    config_path = request.config.getoption("--devnet-config")
    if not config_path:
        pytest.skip("--devnet-config not provided")
    devnet = load_devnet(config_path)
    devnet.verify_chain_ids()
    return devnet


@pytest.fixture
def funded_account(devnet):
    if devnet.funded_account is None:
        pytest.skip("DEVNET_FUNDED_KEY not set")
    return devnet.funded_account


@pytest.fixture(autouse=True)
def _skip_rpc_diff_without_two_nodes(request):
    if not request.node.get_closest_marker("rpc_diff"):
        return
    devnet = request.getfixturevalue("devnet")
    if len(devnet.nodes) < 2:
        pytest.skip("rpc_diff tests need at least 2 configured nodes to compare")
