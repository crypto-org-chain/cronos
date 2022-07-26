from web3 import Web3

from .utils import ADDRS, CONTRACTS, deploy_contract, send_transaction, sign_transaction


def test_pending_transaction_filter(cluster):
    w3: Web3 = cluster.w3
    flt = w3.eth.filter("pending")
    assert flt.get_new_entries() == []

    signed = sign_transaction(w3, {"to": ADDRS["community"], "value": 1000})
    txhash = w3.eth.send_raw_transaction(signed.rawTransaction)
    receipt = w3.eth.wait_for_transaction_receipt(txhash)
    assert receipt.status == 1
    assert txhash in flt.get_new_entries()


def test_block_filter(cronos):
    w3: Web3 = cronos.w3
    flt = w3.eth.filter("latest")
    # new blocks
    signed = sign_transaction(w3, {"to": ADDRS["community"], "value": 1000})
    txhash = w3.eth.send_raw_transaction(signed.rawTransaction)
    receipt = w3.eth.wait_for_transaction_receipt(txhash)
    assert receipt.status == 1
    blocks = flt.get_new_entries()
    assert len(blocks) >= 1


def test_event_log_filter(cronos):
    w3: Web3 = cronos.w3
    mycontract = deploy_contract(w3, CONTRACTS["Greeter"])
    assert "Hello" == mycontract.caller.greet()
    current_height = hex(w3.eth.get_block_number())
    event_filter = mycontract.events.ChangeGreeting.createFilter(
        fromBlock=current_height
    )

    tx = mycontract.functions.setGreeting("world").buildTransaction()
    tx_receipt = send_transaction(w3, tx)
    log = mycontract.events.ChangeGreeting().processReceipt(tx_receipt)[0]
    assert log["event"] == "ChangeGreeting"
    assert tx_receipt.status == 1
    new_entries = event_filter.get_new_entries()
    assert len(new_entries) == 1
    print(f"get event: {new_entries}")
    assert new_entries[0] == log
    assert "world" == mycontract.caller.greet()
