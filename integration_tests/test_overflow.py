from pathlib import Path

import pytest

from .network import setup_custom_cronos
from .utils import ADDRS, KEYS, send_transaction, w3_wait_for_block


@pytest.fixture(scope="module")
def cronos(tmp_path_factory):
    path = tmp_path_factory.mktemp("cronos-overflow")
    yield from setup_custom_cronos(
        path, 27200, Path(__file__).parent / "configs/big_basefee.jsonnet"
    )


def test_basefee_overflow(cronos):
    """
    test if any json-rpc breaks when base fee overflows int64
    """
    w3 = cronos.w3
    w3_wait_for_block(w3, 1)
    assert w3.eth.gas_price > 1000000000000000000
    tx = {
        "from": ADDRS["validator"],
        "to": ADDRS["community"],
        "value": 1,
        "gasPrice": w3.eth.gas_price,
    }
    tx["gas"] = w3.eth.estimate_gas(tx)
    receipt = send_transaction(
        w3,
        tx,
        KEYS["validator"],
    )
    assert receipt.status == 1
    print("receipt", receipt)
