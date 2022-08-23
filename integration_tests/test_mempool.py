from concurrent.futures import ThreadPoolExecutor, as_completed
from pathlib import Path

import pytest
from web3 import Web3
from web3.exceptions import TransactionNotFound
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


@pytest.fixture(scope="module")
def cronos_mempool(tmp_path_factory):
    path = tmp_path_factory.mktemp("cronos-mempool")
    yield from setup_custom_cronos(
        path, 26300, Path(__file__).parent / "configs/long_timeout_commit.jsonnet"
    )


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
    tx = contract.functions.setGreeting("world").buildTransaction()
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
    block_num_0, sended_hash_set = send_txs(w3, cli, to, KEYS.values(), params)

    all_pending = w3.eth.get_filter_changes(filter.filter_id)
    assert len(all_pending) == 0

    block_num_1 = w3.eth.get_block_number()
    assert block_num_1 == block_num_0, "new block is generated, fail"
    print(f"all send tx hash: f{sended_hash_set}")

    # check after max 10 blocks
    for i in range(10):
        all_pending = w3.eth.get_filter_changes(filter.filter_id)
        print(f"all pending tx hash at block {i+block_num_1}: {all_pending}")
        for hash in all_pending:
            sended_hash_set.discard(hash)
        if len(sended_hash_set) == 0:
            break

        # FIXME remove after new design with subscription
        with ThreadPoolExecutor(len(sended_hash_set)) as exec:
            tasks = [
                exec.submit(w3.eth.get_transaction, hash) for hash in sended_hash_set
            ]
            try:
                for future in as_completed(tasks):
                    res = future.result()
                    if res.blockNumber is not None:
                        print(f"{res.hash.hex()} confirmed at {res.blockNumber}")
                        sended_hash_set.discard(res.hash)
            except TransactionNotFound as e:
                print("get txs fail", e)

        if len(sended_hash_set) == 0:
            break
        wait_for_new_blocks(cli, 1, sleep=0.1)
    assert len(sended_hash_set) == 0
