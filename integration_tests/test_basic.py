from .utils import ADDRS


def test_basic(w3):
    assert w3.eth.chain_id == 777
    w3.eth.get_balance(ADDRS["community"]) == 0
