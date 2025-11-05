from pathlib import Path

import pytest
from web3 import Web3

from .network import setup_custom_cronos
from .utils import (
    ADDRS,
    CONTRACTS,
    KEYS,
    deploy_contract,
    get_account_nonce,
    replace_transaction,
    send_transaction,
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


@pytest.mark.flaky(max_runs=3)
def test_mempool_nonce(cronos_mempool):
    """
    test the nonce logic in check-tx after new block is created.

    we'll insert several transactions into mempool with increasing nonces,
    the tx body is so large that they won't be included in next block at the same time,
    then we'll try to send a new tx with local nonce to see if it still get accepted
    even if check-tx state get reset.

    the expected behavior is when mempool.recheck=true, this test should pass, because
    although check-tx state get reset when new blocks generated, but recheck logic will
    bring it back in sync with pending txs, so the client can keep sending new
    transactions with local nonce.
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
            "gas": 10021000,
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
    assert orig_nonce + (new_height - height) == w3.eth.get_transaction_count(sender)
    assert orig_nonce + 3 == local_nonce

    for i in range(3):
        # send a new tx with the next nonce
        txhash = send_with_nonce(local_nonce)
        print(f"txhash: {txhash.hex()}")
        local_nonce += 1

        new_height = wait_for_new_blocks(cli, 1, sleep=0.1)
        assert orig_nonce + (new_height - height) == w3.eth.get_transaction_count(
            sender
        )
        assert orig_nonce + 4 + i == local_nonce


@pytest.mark.flaky(max_runs=3)
def test_tx_replacement(cronos_mempool):
    w3 = cronos_mempool.w3
    base_fee = w3.eth.get_block("latest")["baseFeePerGas"]
    priority_fee = w3.eth.max_priority_fee
    nonce = get_account_nonce(w3)
    # replace with less than 10% bump, should fail
    with pytest.raises(ValueError) as exc:
        _ = replace_transaction(
            w3,
            {
                "to": ADDRS["community"],
                "value": 1,
                "maxFeePerGas": base_fee + priority_fee,
                "maxPriorityFeePerGas": priority_fee,
                "nonce": nonce,
                "from": ADDRS["validator"],
            },
            {
                "to": ADDRS["community"],
                "value": 2,
                "maxFeePerGas": int((base_fee + priority_fee) * 1.05),  # +5% bump
                "maxPriorityFeePerGas": int(priority_fee * 1.05),
                "nonce": nonce,
                "from": ADDRS["validator"],
            },
            KEYS["validator"],
        )["transactionHash"]
    assert "tx doesn't fit the replacement rule" in str(exc)

    wait_for_new_blocks(cronos_mempool.cosmos_cli(), 1)
    nonce = get_account_nonce(w3)
    initial_balance = w3.eth.get_balance(ADDRS["community"])
    # replace with more than 10% bump, should succeed
    txhash = replace_transaction(
        w3,
        {
            "to": ADDRS["community"],
            "value": 3,
            "maxFeePerGas": base_fee + priority_fee,
            "maxPriorityFeePerGas": priority_fee,
            "nonce": nonce,
            "from": ADDRS["validator"],
        },
        {
            "to": ADDRS["community"],
            "value": 5,
            "maxFeePerGas": int((base_fee + priority_fee) * 1.15),  # +15% bump
            "maxPriorityFeePerGas": int(priority_fee * 1.15),
            "nonce": nonce,
            "from": ADDRS["validator"],
        },
        KEYS["validator"],
    )["transactionHash"]
    tx1 = w3.eth.get_transaction(txhash)
    assert tx1["transactionIndex"] == 0
    assert w3.eth.get_balance(ADDRS["community"]) == initial_balance + 5

    # check that already accepted transaction cannot be replaced
    txhash_noreplacemenet = send_transaction(
        w3,
        {
            "to": ADDRS["community"],
            "value": 10,
            "maxFeePerGas": base_fee + priority_fee,
            "maxPriorityFeePerGas": priority_fee,
        },
        KEYS["validator"],
    )["transactionHash"]
    tx2 = w3.eth.get_transaction(txhash_noreplacemenet)
    assert tx2["transactionIndex"] == 0

    with pytest.raises(ValueError) as exc:
        w3.eth.replace_transaction(
            txhash_noreplacemenet,
            {
                "to": ADDRS["community"],
                "value": 15,
                "maxFeePerGas": int((base_fee + priority_fee) * 1.15),  # +15% bump
                "maxPriorityFeePerGas": int(priority_fee * 1.15),
            },
        )
    assert "has already been mined" in str(exc)
