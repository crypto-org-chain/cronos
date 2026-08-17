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


def test_send_conn_per_host_defaults_to_200():
    assert Config.model_validate(BASE_CONFIG).send_conn_per_host == 200


def test_send_workers_defaults_to_1():
    assert Config.model_validate(BASE_CONFIG).send_workers == 1


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


def test_endpoint_candidates_default_to_single_url():
    endpoint = Config.model_validate(BASE_CONFIG).primary
    assert endpoint.rpc_candidates == ["http://127.0.0.1:26657"]
    assert endpoint.json_rpc_candidates == ["http://127.0.0.1:26651"]


def test_endpoint_candidates_include_pool():
    cfg = Config.model_validate(
        {
            **BASE_CONFIG,
            "endpoints": [
                {
                    **BASE_CONFIG["endpoints"][0],
                    "rpc_pool": ["http://127.0.0.1:18658", "http://127.0.0.1:18659"],
                    "json_rpc_pool": ["http://127.0.0.1:18546"],
                }
            ],
        }
    )
    endpoint = cfg.primary
    assert endpoint.rpc_candidates == [
        "http://127.0.0.1:26657",
        "http://127.0.0.1:18658",
        "http://127.0.0.1:18659",
    ]
    assert endpoint.json_rpc_candidates == [
        "http://127.0.0.1:26651",
        "http://127.0.0.1:18546",
    ]


def test_config_candidates_flatten_across_endpoints_and_pools():
    # Load-sending must round-robin across every node in the cluster, not
    # just the primary's own tunnel pool - regression: a prior change
    # narrowed this to cfg.primary.rpc_candidates, silently funneling all
    # load to node0 on any multi-endpoint config.
    cfg = Config.model_validate(
        {
            **BASE_CONFIG,
            "endpoints": [
                {
                    "name": "node0",
                    "rpc": "http://node0",
                    "json_rpc": "http://node0-evm",
                    "rpc_pool": ["http://node0-pool1"],
                },
                {
                    "name": "node1",
                    "rpc": "http://node1",
                    "json_rpc": "http://node1-evm",
                },
            ],
        }
    )

    assert cfg.rpc_candidates == ["http://node0", "http://node0-pool1", "http://node1"]
    assert cfg.json_rpc_candidates == ["http://node0-evm", "http://node1-evm"]

