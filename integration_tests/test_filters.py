from pathlib import Path

import pytest
from web3 import Web3

from .network import setup_custom_cronos
from .utils import (
    ADDRS,
    CONTRACTS,
    deploy_contract,
    send_transaction,
    wait_for_new_blocks,
)


@pytest.fixture(scope="module")
def flood_cronos(tmp_path_factory):
    # Pending filter reads CometBFT UnconfirmedTxs, empty under the default
    # mempool.type=app; boot a FLOOD network for it. Port 27110 is free.
    path = tmp_path_factory.mktemp("filters-flood")
    yield from setup_custom_cronos(
        path, 27110, Path(__file__).parent / "configs/enable-indexer-flood.jsonnet"
    )


@pytest.fixture(params=["flood_cronos", "geth"])
def pending_filter_provider(request, flood_cronos, geth):
    return flood_cronos if request.param == "flood_cronos" else geth


def test_pending_transaction_filter(pending_filter_provider):
    w3: Web3 = pending_filter_provider.w3
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
