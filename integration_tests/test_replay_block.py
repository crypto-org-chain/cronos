from pathlib import Path

import pytest
import web3
from web3._utils.method_formatters import receipt_formatter
from web3.datastructures import AttributeDict

from .network import setup_custom_cronos
from .utils import (
    ADDRS,
    CONTRACTS,
    KEYS,
    deploy_contract,
    send_raw_transactions,
    sign_transaction,
    wait_for_new_blocks,
)

pytestmark = pytest.mark.slow


@pytest.fixture(scope="module")
def custom_cronos(tmp_path_factory):
    path = tmp_path_factory.mktemp("cronos")
    yield from setup_custom_cronos(
        path, 26000, Path(__file__).parent / "configs/low_block_gas_limit.jsonnet"
    )


@pytest.mark.skip(reason="block gas limit is disabled")
def test_block_overflow(custom_cronos):
    w3: web3.Web3 = custom_cronos.w3
    contract = deploy_contract(
        w3,
        CONTRACTS["TestMessageCall"],
        key=KEYS["community"],
    )
    iterations = 400
    gas_limit = 800000
    gas_price = 200000000000
    names = ["validator", "validator2"]
    addrs = [ADDRS[names[0]], ADDRS[names[1]]]
    keys = [KEYS[names[0]], KEYS[names[1]]]

    gas_limits = {}
    blks = []
    success = None
    fail = None
    for i in range(10):
        raw_transactions = []
        nonces = {}
        begin_balances = {}
        for i, key_from in enumerate(keys):
            addr = addrs[i]
            nonces[addr] = w3.eth.get_transaction_count(addr)
            begin_balances[addr] = w3.eth.get_balance(addr)
            gas_limits[addr] = gas_limit + i
            tx = contract.functions.test(iterations).build_transaction(
                {
                    "nonce": nonces[addr],
                    "gas": gas_limits[addr],
                    "gasPrice": gas_price,
                }
            )
            signed = sign_transaction(w3, tx, key_from)
            raw_transactions.append(signed.rawTransaction)

        # wait block update
        block_num_0 = wait_for_new_blocks(custom_cronos.cosmos_cli(), 1, sleep=0.1)
        print(f"block number start: {block_num_0}")
        sended_hash_set = send_raw_transactions(w3, raw_transactions)
        for h in sended_hash_set:
            res = w3.eth.wait_for_transaction_receipt(h)
            addr = res["from"]

            # check sender's nonce is increased once, which means both txs are executed.
            assert nonces[addr] + 1 == w3.eth.get_transaction_count(addr)
            # check sender's balance is deducted as expected
            diff = begin_balances[addr] - w3.eth.get_balance(addr)
            assert res["gasUsed"] * gas_price == diff

            blks.append(res.blockNumber)
            if res.status == 1:
                success = res
            elif res.status == 0:
                fail = res

        if all(blk == blks[0] for blk in blks):
            break
        print(
            "tx1 and tx2 are included in two different blocks, retry now.",
            blks,
        )
    else:
        assert False, "timeout"

    # the first tx succeeds.
    assert success.status == 1
    assert success.gasUsed < gas_limits[success["from"]]
    assert success.cumulativeGasUsed == success.gasUsed

    # the second tx should fail and cost the whole gasLimit
    assert fail.status == 0
    assert fail.gasUsed == gas_limits[fail["from"]]
    assert fail.cumulativeGasUsed == success.cumulativeGasUsed + fail.gasUsed

    # check get block apis
    assert w3.eth.get_block(success.blockNumber).transactions == [
        success.transactionHash,
        fail.transactionHash,
    ]
    res = w3.eth.get_transaction_by_block(fail.blockNumber, fail.transactionIndex)
    assert res.hash == fail.transactionHash

    rsp = w3.provider.make_request(
        "cronos_replayBlock", [hex(success.blockNumber), False]
    )
    assert "error" not in rsp, rsp["error"]
    assert 2 == len(rsp["result"])

    # check the replay receipts are the same
    replay_receipts = [AttributeDict(receipt_formatter(item)) for item in rsp["result"]]
    assert replay_receipts[0].gasUsed == replay_receipts[1].gasUsed == success.gasUsed
    assert replay_receipts[0].status == replay_receipts[1].status == success.status
    assert (
        replay_receipts[0].logsBloom
        == replay_receipts[1].logsBloom
        == success.logsBloom
    )
    assert replay_receipts[0].cumulativeGasUsed == success.cumulativeGasUsed
    assert replay_receipts[1].cumulativeGasUsed == success.cumulativeGasUsed * 2

    # check the postUpgrade mode
    rsp = w3.provider.make_request(
        "cronos_replayBlock", [hex(success.blockNumber), True]
    )
    assert "error" not in rsp, rsp["error"]
    assert 2 == len(rsp["result"])
    replay_receipts = [AttributeDict(receipt_formatter(item)) for item in rsp["result"]]
    assert replay_receipts[1].status == 0
    assert replay_receipts[1].gasUsed == gas_limits[replay_receipts[1]["from"]]
