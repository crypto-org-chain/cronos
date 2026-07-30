from dataclasses import dataclass

import pytest
from eth_account import Account
from web3 import Web3

from .config import funded_key, load_config


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


@dataclass
class Node:
    name: str
    w3: Web3


@dataclass
class Devnet:
    nodes: list[Node]
    funded_account: Account | None


def load_devnet(config_path: str) -> Devnet:
    cfg = load_config(config_path)
    nodes = [Node(n.name, Web3(Web3.HTTPProvider(n.json_rpc))) for n in cfg.nodes]
    key = funded_key()
    return Devnet(nodes, Account.from_key(key) if key else None)


@pytest.fixture(scope="session")
def devnet(request):
    config_path = request.config.getoption("--devnet-config")
    if not config_path:
        pytest.skip("--devnet-config not provided")
    return load_devnet(config_path)


@pytest.fixture(autouse=True)
def _skip_rpc_diff_without_two_nodes(request):
    if not request.node.get_closest_marker("rpc_diff"):
        return
    devnet = request.getfixturevalue("devnet")
    if len(devnet.nodes) < 2:
        pytest.skip("rpc_diff tests need at least 2 configured nodes to compare")
