from web3 import Web3

from .utils import ADDRS, sign_transaction


def test_pending_transaction_filter(cluster):
    w3: Web3 = cluster.w3
    flt = w3.eth.filter("pending")
    assert flt.get_new_entries() == []

    signed = sign_transaction(w3, {"to": ADDRS["community"], "value": 1000})
    txhash = w3.eth.send_raw_transaction(signed.rawTransaction)
    receipt = w3.eth.wait_for_transaction_receipt(txhash)
    assert receipt.status == 1
    assert flt.get_new_entries() == [txhash]
