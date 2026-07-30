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


def test_weighted_mix_requires_mix_weights():
    with pytest.raises(ValidationError, match="mix_weights must be set"):
        Config.model_validate({**BASE_CONFIG, "tx_type": "weighted-mix"})


def test_weighted_mix_rejects_unknown_tx_type_in_mix_weights():
    with pytest.raises(ValidationError, match="unknown tx types"):
        Config.model_validate(
            {
                **BASE_CONFIG,
                "tx_type": "weighted-mix",
                "mix_weights": {"not-a-real-type": 1.0},
            }
        )


def test_weighted_mix_rejects_all_zero_weights():
    with pytest.raises(ValidationError, match="positive total"):
        Config.model_validate(
            {
                **BASE_CONFIG,
                "tx_type": "weighted-mix",
                "mix_weights": {"erc20-transfer-hot": 0.0, "nft-mint": 0.0},
            }
        )


def test_weighted_mix_rejects_negative_weights():
    with pytest.raises(ValidationError, match="negative weights"):
        Config.model_validate(
            {
                **BASE_CONFIG,
                "tx_type": "weighted-mix",
                "mix_weights": {"erc20-transfer-hot": 5.0, "nft-mint": -3.0},
            }
        )


def test_weighted_mix_accepts_valid_mix_weights():
    cfg = Config.model_validate(
        {
            **BASE_CONFIG,
            "tx_type": "weighted-mix",
            "mix_weights": {"erc20-transfer-hot": 0.5, "uniswap-swap": 0.5},
        }
    )
    assert cfg.mix_weights == {"erc20-transfer-hot": 0.5, "uniswap-swap": 0.5}
