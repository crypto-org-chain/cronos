from pathlib import Path

import pytest
import web3
from web3._utils.method_formatters import receipt_formatter
from web3.datastructures import AttributeDict

from .network import setup_custom_cronos
from .utils import ADDRS, CONTRACTS, KEYS, deploy_contract, sign_transaction

pytestmark = pytest.mark.slow


@pytest.fixture(scope="module")
def custom_cronos(tmp_path_factory):
    path = tmp_path_factory.mktemp("cronos")
    yield from setup_custom_cronos(
        path, 26000, Path(__file__).parent / "configs/low_block_gas_limit.jsonnet"
    )


def test_block_overflow(custom_cronos):
    w3: web3.Web3 = custom_cronos.w3
    contract = deploy_contract(
        w3,
        CONTRACTS["TestMessageCall"],
        key=KEYS["community"],
    )
    iterations = 400
    gas_limit = 800000
    gas_price = 100000000000
    for i in range(10):
        nonce = w3.eth.get_transaction_count(ADDRS["validator"])
        begin_balance = w3.eth.get_balance(ADDRS["validator"])
        txs = [
            contract.functions.test(iterations).build_transaction(
                {
                    "nonce": nonce,
                    "gas": gas_limit,
                    "gasPrice": gas_price,
                }
            ),
            contract.functions.test(iterations).build_transaction(
                {
                    "nonce": nonce + 1,
                    "gas": gas_limit + 1,
                    "gasPrice": gas_price,
                }
            ),
        ]
        txhashes = [
            w3.eth.send_raw_transaction(sign_transaction(w3, tx).rawTransaction)
            for tx in txs
        ]
        receipt1 = w3.eth.wait_for_transaction_receipt(txhashes[0])
        # the tx2 should be included in json-rpc response too.
        receipt2 = w3.eth.wait_for_transaction_receipt(txhashes[1], timeout=10)
        if receipt1.blockNumber == receipt2.blockNumber:
            break
        print(
            "tx1 and tx2 are included in two different blocks, retry now.",
            receipt1.blockNumber,
            receipt2.blockNumber,
        )
    else:
        assert False, "timeout"

    # the first tx succeds.
    assert receipt1.status == 1
    assert receipt1.gasUsed < gas_limit
    assert receipt1.cumulativeGasUsed == receipt1.gasUsed

    # the second tx should fail and cost the whole gasLimit
    assert receipt2.status == 0
    assert receipt2.gasUsed == gas_limit + 1
    assert receipt2.cumulativeGasUsed == receipt1.cumulativeGasUsed + gas_limit + 1

    # check get block apis
    assert w3.eth.get_block(receipt1.blockNumber).transactions == [
        receipt1.transactionHash,
        receipt2.transactionHash,
    ]
    assert (
        w3.eth.get_transaction_by_block(receipt1.blockNumber, 1).hash
        == receipt2.transactionHash
    )

    # check sender's nonce is increased twice, which means both txs are executed.
    assert nonce + 2 == w3.eth.get_transaction_count(ADDRS["validator"])
    # check sender's balance is deducted as expected
    assert receipt2.cumulativeGasUsed * gas_price == begin_balance - w3.eth.get_balance(
        ADDRS["validator"]
    )

    rsp = w3.provider.make_request(
        "cronos_replayBlock", [hex(receipt1.blockNumber), False]
    )
    assert "error" not in rsp, rsp["error"]
    assert 2 == len(rsp["result"])

    # check the replay receipts are the same
    replay_receipts = [AttributeDict(receipt_formatter(item)) for item in rsp["result"]]
    assert replay_receipts[0].gasUsed == replay_receipts[1].gasUsed == receipt1.gasUsed
    assert replay_receipts[0].status == replay_receipts[1].status == receipt1.status
    assert (
        replay_receipts[0].logsBloom
        == replay_receipts[1].logsBloom
        == receipt1.logsBloom
    )
    assert replay_receipts[0].cumulativeGasUsed == receipt1.cumulativeGasUsed
    assert replay_receipts[1].cumulativeGasUsed == receipt1.cumulativeGasUsed * 2

    # check the postUpgrade mode
    rsp = w3.provider.make_request(
        "cronos_replayBlock", [hex(receipt1.blockNumber), True]
    )
    assert "error" not in rsp, rsp["error"]
    assert 2 == len(rsp["result"])
    replay_receipts = [AttributeDict(receipt_formatter(item)) for item in rsp["result"]]
    assert replay_receipts[1].status == 0
    assert replay_receipts[1].gasUsed == gas_limit + 1
