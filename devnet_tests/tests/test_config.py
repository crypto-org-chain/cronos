from types import SimpleNamespace

import pytest
from pydantic import ValidationError

from devnet_tests.config import Config
from devnet_tests.devnet import Devnet, Node

BASE_CONFIG = {
    "nodes": [
        {"name": "v1.7.8", "rpc": "http://a:26657", "json_rpc": "http://a:8545"},
        {"name": "v1.8", "rpc": "http://b:26657", "json_rpc": "http://b:8545"},
    ],
    "chain_id": 777,
}


def test_loads_valid_config():
    cfg = Config.model_validate(BASE_CONFIG)
    assert [n.name for n in cfg.nodes] == ["v1.7.8", "v1.8"]
    assert cfg.chain_id == 777


def test_requires_at_least_one_node():
    with pytest.raises(ValidationError):
        Config.model_validate({**BASE_CONFIG, "nodes": []})


@pytest.mark.parametrize("field", ["name", "rpc", "json_rpc"])
def test_rejects_two_nodes_sharing_an_endpoint(field):
    # One node under two names makes every cross-node diff compare it to itself.
    nodes = [dict(BASE_CONFIG["nodes"][0]), dict(BASE_CONFIG["nodes"][1])]
    nodes[1][field] = nodes[0][field]

    with pytest.raises(ValidationError, match=f"duplicate {field}"):
        Config.model_validate({**BASE_CONFIG, "nodes": nodes})


def _node(name, chain_id):
    return Node(name, SimpleNamespace(eth=SimpleNamespace(chain_id=chain_id)), "rpc")


def test_verify_chain_ids_accepts_nodes_on_the_configured_chain():
    devnet = Devnet([_node("a", 777), _node("b", 777)], None, chain_id=777)
    devnet.verify_chain_ids()


def test_verify_chain_ids_rejects_a_node_on_another_chain():
    devnet = Devnet([_node("a", 777), _node("b", 25)], None, chain_id=777)

    with pytest.raises(ValueError, match="b reports chain_id 25"):
        devnet.verify_chain_ids()


def test_verify_chain_ids_is_a_noop_without_an_expected_chain_id():
    Devnet([_node("a", 25)], None).verify_chain_ids()
