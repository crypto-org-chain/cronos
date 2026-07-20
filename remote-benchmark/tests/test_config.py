import pytest
from pydantic import ValidationError

from remote_benchmark.config import Config

BASE_CONFIG = {
    "endpoints": [
        {
            "name": "node0",
            "rpc": "http://127.0.0.1:26657",
            "json_rpc": "http://127.0.0.1:26651",
        }
    ],
    "chain_id": 777,
}


def test_sender_strategy_defaults_to_reuse():
    assert Config.model_validate(BASE_CONFIG).sender_strategy == "reuse"


def test_sender_strategy_rejects_unknown_value():
    with pytest.raises(ValidationError):
        Config.model_validate({**BASE_CONFIG, "sender_strategy": "random"})
