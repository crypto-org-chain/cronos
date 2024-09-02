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
    assert len(all_pending) == 4

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


@pytest.mark.flaky(max_runs=5)
def test_tx_replacement(cronos_mempool):
    w3: Web3 = cronos_mempool.w3
    account = "community"
    nonce = w3.eth.get_transaction_count(ADDRS[account])
    gas_price = w3.eth.gas_price
    reduction = 1000000
    # the second tx should replace the first tx with higher priority,
    # but the third one shouldn't replace the second one.
    prices = [
        gas_price,
        gas_price + 2 * reduction,
        gas_price + reduction,
    ]
    txs = [
        sign_transaction(
            w3,
            {
                "to": ADDRS[account],
                "value": 1,
                "gas": 21000,
                "gasPrice": price,
                "nonce": nonce,
            },
            KEYS[account],
        )
        for price in prices
    ]

    txhashes = [w3.eth.send_raw_transaction(tx.rawTransaction) for tx in txs]
    receipt = w3.eth.wait_for_transaction_receipt(txhashes[1])
    assert receipt.status == 1
