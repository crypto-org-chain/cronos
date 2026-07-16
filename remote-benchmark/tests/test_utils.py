from remote_benchmark.utils import bech32_to_eth, eth_to_bech32, split, split_batch


def test_bech32_roundtrip():
    addr = "0x1234567890123456789012345678901234567890"
    bech = eth_to_bech32(addr)
    assert bech32_to_eth(bech) == addr.lower()


def test_split():
    assert split(10, 3) == [(0, 4), (4, 7), (7, 10)]


def test_split_batch():
    assert split_batch(10, 3) == [(0, 3), (3, 6), (6, 9), (9, 10)]
