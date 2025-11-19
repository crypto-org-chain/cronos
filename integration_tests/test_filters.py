from web3 import Web3

from .utils import (
    ADDRS,
    CONTRACTS,
    deploy_contract,
    send_transaction,
    wait_for_new_blocks,
)


def test_pending_transaction_filter(cluster):
    w3: Web3 = cluster.w3
    flt = w3.eth.filter("pending")
    assert flt.get_new_entries() == []
    receipt = send_transaction(w3, {"to": ADDRS["community"], "value": 1000})
    assert receipt.status == 1
    assert receipt["transactionHash"] in flt.get_new_entries()


def test_block_filter(cronos):
    w3: Web3 = cronos.w3
    flt = w3.eth.filter("latest")
    # new blocks
    wait_for_new_blocks(cronos.cosmos_cli(), 1, sleep=0.1)
    receipt = send_transaction(w3, {"to": ADDRS["community"], "value": 1000})
    assert receipt.status == 1
    blocks = flt.get_new_entries()
    assert len(blocks) >= 1


def test_event_log_filter(cronos):
    w3: Web3 = cronos.w3
    mycontract = deploy_contract(w3, CONTRACTS["Greeter"])
    assert "Hello" == mycontract.caller.greet()
    current_height = hex(w3.eth.get_block_number())
    event_filter = mycontract.events.ChangeGreeting.create_filter(
        from_block=current_height
    )

    tx = mycontract.functions.setGreeting("world").build_transaction()
    tx_receipt = send_transaction(w3, tx)
    log = mycontract.events.ChangeGreeting().process_receipt(tx_receipt)[0]
    assert log["event"] == "ChangeGreeting"
    assert tx_receipt.status == 1
    new_entries = event_filter.get_new_entries()
    assert len(new_entries) == 1
    print(f"get event: {new_entries}")
    assert new_entries[0] == log
    assert "world" == mycontract.caller.greet()
    # without new txs since last call
    assert event_filter.get_new_entries() == []
    assert event_filter.get_all_entries() == new_entries
    # Uninstall
    assert w3.eth.uninstall_filter(event_filter.filter_id)
    assert not w3.eth.uninstall_filter(event_filter.filter_id)
