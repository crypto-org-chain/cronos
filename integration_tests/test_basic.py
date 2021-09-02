from .utils import ADDRS


def test_basic(cluster):
    w3 = cluster.w3
    assert w3.eth.chain_id == 777
    assert w3.eth.get_balance(ADDRS["community"]) == 10000000000000000000000
