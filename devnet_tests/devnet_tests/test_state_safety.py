import pytest

from .state_safety import check_app_hash_agreement, historical_query_soak

HISTORICAL_LOOKBACK = 100
SOAK_ITERATIONS = 200
APP_HASH_WINDOW = 20


@pytest.mark.rpc_diff
def test_app_hash_agreement(devnet):
    end = min(node.w3.eth.block_number for node in devnet.nodes)
    start = max(end - APP_HASH_WINDOW, 1)
    assert check_app_hash_agreement(devnet.nodes, start, end) == []


def test_historical_query_soak(devnet, funded_account):
    w3 = devnet.nodes[0].w3
    height = w3.eth.block_number - HISTORICAL_LOOKBACK
    if height < 1:
        pytest.skip("chain isn't tall enough yet for a historical query soak")
    result = historical_query_soak(w3, funded_account.address, height, SOAK_ITERATIONS)
    assert result.errors == []
