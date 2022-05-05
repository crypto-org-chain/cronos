from pathlib import Path

import pytest
import web3
from pystarport import ports
from web3._utils.method_formatters import receipt_formatter
from web3.datastructures import AttributeDict

from .network import setup_custom_cronos
from .utils import (
    ADDRS,
    KEYS,
    deploy_contract,
    sign_transaction,
    supervisorctl,
    wait_for_port,
)


@pytest.fixture(scope="module")
def custom_cronos(tmp_path_factory):
    path = tmp_path_factory.mktemp("cronos")
    yield from setup_custom_cronos(
        path, 26000, Path(__file__).parent / "configs/low_block_gas_limit.yaml"
    )


def test_replay_block(custom_cronos):
    w3: web3.Web3 = custom_cronos.w3
    cli = custom_cronos.cosmos_cli()
    begin_height = cli.block_height()
    contract = deploy_contract(
        w3,
        Path(__file__).parent
        / "contracts/artifacts/contracts/TestMessageCall.sol/TestMessageCall.json",
        key=KEYS["community"],
    )
    iterations = 400
    gas_limit = 800000
    for i in range(10):
        nonce = w3.eth.get_transaction_count(ADDRS["validator"])
        txs = [
            contract.functions.test(iterations).buildTransaction(
                {
                    "nonce": nonce,
                    "gas": gas_limit,
                }
            ),
            contract.functions.test(iterations).buildTransaction(
                {
                    "nonce": nonce + 1,
                    "gas": gas_limit,
                }
            ),
        ]
        txhashes = [
            w3.eth.send_raw_transaction(sign_transaction(w3, tx).rawTransaction)
            for tx in txs
        ]
        receipt1 = w3.eth.wait_for_transaction_receipt(txhashes[0])
        try:
            receipt2 = w3.eth.wait_for_transaction_receipt(txhashes[1], timeout=10)
        except web3.exceptions.TimeExhausted:
            # expected exception, tx2 is included but failed.
            receipt2 = None
            break
        if receipt1.blockNumber == receipt2.blockNumber:
            break
        print(
            "tx1 and tx2 are included in two different blocks, retry now.",
            receipt1.blockNumber,
            receipt2.blockNumber,
        )
    else:
        assert False, "timeout"
    assert not receipt2
    # check sender's nonce is increased twice, which means both txs are executed.
    assert nonce + 2 == w3.eth.get_transaction_count(ADDRS["validator"])
    rsp = w3.provider.make_request(
        "cronos_replayBlock", [hex(receipt1.blockNumber), False]
    )
    assert "error" not in rsp, rsp["error"]
    assert 2 == len(rsp["result"])

    # check the replay receipts are the same
    replay_receipts = [AttributeDict(receipt_formatter(item)) for item in rsp["result"]]
    # assert replay_receipts[0].gasUsed==replay_receipts[1].gasUsed == receipt1.gasUsed
    assert replay_receipts[0].status == replay_receipts[1].status == receipt1.status
    assert (
        replay_receipts[0].logsBloom
        == replay_receipts[1].logsBloom
        == receipt1.logsBloom
    )
    # assert replay_receipts[0].cumulativeGasUsed == receipt1.cumulativeGasUsed
    # assert replay_receipts[1].cumulativeGasUsed == receipt1.cumulativeGasUsed * 2

    # check the postUpgrade mode
    rsp = w3.provider.make_request(
        "cronos_replayBlock", [hex(receipt1.blockNumber), True]
    )
    assert "error" not in rsp, rsp["error"]
    assert 2 == len(rsp["result"])
    replay_receipts = [AttributeDict(receipt_formatter(item)) for item in rsp["result"]]
    assert replay_receipts[1].status == 0
    assert replay_receipts[1].gasUsed == gas_limit

    # patch the unlucky tx with the new cli command
    # stop the node0
    end_height = cli.block_height()
    supervisorctl(custom_cronos.base_dir / "../tasks.ini", "stop", "cronos_777-1-node0")
    results = custom_cronos.cosmos_cli().fix_unlucky_tx(begin_height, end_height)
    # the second tx is patched
    assert results[0].hash == txhashes[1]
    # start the node0 again
    supervisorctl(
        custom_cronos.base_dir / "../tasks.ini", "start", "cronos_777-1-node0"
    )
    # wait for json-rpc port
    wait_for_port(ports.evmrpc_port(custom_cronos.base_port(0)))
    w3: web3.Web3 = custom_cronos.w3
    receipt = w3.eth.get_transaction_receipt(txhashes[1])
    # check the receipt is the same as a successful one
    assert receipt.status == 1
    assert receipt.gasUsed == receipt1.gasUsed
    assert receipt.logsBloom == receipt1.logsBloom
    assert receipt.logs == receipt1.logs
