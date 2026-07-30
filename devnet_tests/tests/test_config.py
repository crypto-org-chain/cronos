import pytest
from pydantic import ValidationError

from devnet_tests.config import Config

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
