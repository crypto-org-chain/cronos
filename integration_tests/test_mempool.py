from pathlib import Path

import pytest
from web3 import Web3

from .network import setup_custom_cronos
from .utils import (
    ADDRS,
    CONTRACTS,
    KEYS,
    deploy_contract,
    send_txs,
    sign_transaction,
    wait_for_new_blocks,
)

pytestmark = pytest.mark.slow


@pytest.fixture(scope="module")
def cronos_mempool(tmp_path_factory):
    path = tmp_path_factory.mktemp("cronos-mempool")
    yield from setup_custom_cronos(
        path, 26300, Path(__file__).parent / "configs/long_timeout_commit.jsonnet"
    )


@pytest.mark.flaky(max_runs=5)
def test_mempool(cronos_mempool):
    w3: Web3 = cronos_mempool.w3
    filter = w3.eth.filter("pending")
    assert filter.get_new_entries() == []

    cli = cronos_mempool.cosmos_cli(0)
    # test contract
    wait_for_new_blocks(cli, 1, sleep=0.1)
    block_num_2 = w3.eth.get_block_number()
    print(f"block number contract begin at height: {block_num_2}")
    contract = deploy_contract(w3, CONTRACTS["Greeter"])
    tx = contract.functions.setGreeting("world").build_transaction()
    signed = sign_transaction(w3, tx)
    txhash = w3.eth.send_raw_transaction(signed.rawTransaction)
    w3.eth.wait_for_transaction_receipt(txhash)
    # check tx in mempool
    new_txs = filter.get_new_entries()
    assert txhash in new_txs

    greeter_call_result = contract.caller.greet()
    assert "world" == greeter_call_result

    # check mempool
    all_pending = w3.eth.get_filter_changes(filter.filter_id)
    print(f"all pending tx hash after block: {all_pending}")
    assert len(all_pending) == 0

    to = ADDRS["community"]
    params = {"gasPrice": w3.eth.gas_price}
    block_num_0, sended_hash_set = send_txs(
        w3, cli, to, [v for k, v in KEYS.items() if k != "signer1"], params
    )
    print(f"all send tx hash: {sended_hash_set} at {block_num_0}")

    all_pending = w3.eth.get_filter_changes(filter.filter_id)
    assert len(all_pending) == len(KEYS.items()) - 1

    block_num_1 = w3.eth.get_block_number()
    print(f"block_num_1 {block_num_1}")

    # check after max 10 blocks
    for i in range(10):
        all_pending = w3.eth.get_filter_changes(filter.filter_id)
        if len(all_pending) == 0:
            break
        wait_for_new_blocks(cli, 1, sleep=0.1)
    assert len(all_pending) == 0


def test_blocked_address(cronos_mempool):
    cli = cronos_mempool.cosmos_cli(0)
    rsp = cli.transfer("signer1", cli.address("validator"), "1basecro")
    assert rsp["code"] != 0
    assert "signer is blocked" in rsp["raw_log"]


def test_mempool_nonce(cronos_mempool):
    """
    test the nonce logic in check-tx after new block is created.

    we'll insert several transactions into mempool with increasing nonces,
    the tx body is so large that they won't be included in next block at the same time,
    then we'll try to send a new tx with local nonce to see if it still get accepted even if
    check-tx state get reset.

    the expected behavior is when mempool.recheck=true, this test should pass, because although check-tx state get reset when new blocks generated, but recheck logic will bring it back in sync with pending txs, so the client can keep sending new transactions with local nonce.
    """
    w3: Web3 = cronos_mempool.w3
    cli = cronos_mempool.cosmos_cli(0)
    wait_for_new_blocks(cli, 1, sleep=0.1)
    sender = ADDRS["validator"]
    orig_nonce = w3.eth.get_transaction_count(sender)
    height = w3.eth.get_block_number()
    local_nonce = orig_nonce
    tx_bytes = 1000000  # can only include one tx at a time

    def send_with_nonce(nonce):
        tx = {
            "to": ADDRS["community"],
            "value": 1,
            "gas": 4121000,
            "data": "0x" + "00" * tx_bytes,
            "nonce": nonce,
        }
        signed = sign_transaction(w3, tx, KEYS["validator"])
        txhash = w3.eth.send_raw_transaction(signed.rawTransaction)
        return txhash

    for i in range(3):
        txhash = send_with_nonce(local_nonce)
        print(f"txhash: {txhash.hex()}")
        local_nonce += 1

    new_height = wait_for_new_blocks(cli, 1, sleep=0.1)
    assert orig_nonce + (new_height-height) == w3.eth.get_transaction_count(sender)
    assert orig_nonce + 3 == local_nonce

    for i in range(3):
        # send a new tx with the next nonce
        txhash = send_with_nonce(local_nonce)
        print(f"txhash: {txhash.hex()}")
        local_nonce += 1

        new_height = wait_for_new_blocks(cli, 1, sleep=0.1)
        assert orig_nonce + (new_height-height) == w3.eth.get_transaction_count(sender)
        assert orig_nonce + 4 + i == local_nonce
