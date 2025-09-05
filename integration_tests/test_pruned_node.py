from pathlib import Path

import pytest
from eth_bloom import BloomFilter
from eth_utils import abi, big_endian_to_int
from hexbytes import HexBytes
from web3.datastructures import AttributeDict

from .network import setup_custom_cronos
from .utils import (
    ADDRS,
    CONTRACTS,
    KEYS,
    deploy_contract,
    sign_transaction,
    wait_for_new_blocks,
)

pytestmark = pytest.mark.slow


@pytest.fixture(scope="module")
def cronos(request, tmp_path_factory):
    """start-cronos
    params: enable_auto_deployment
    """
    yield from setup_custom_cronos(
        tmp_path_factory.mktemp("pruned"),
        26900,
        Path(__file__).parent / "configs/pruned-node.jsonnet",
    )


def test_pruned_node(cronos):
    """
    test basic json-rpc apis works in pruned node
    """
    w3 = cronos.w3
    erc20 = deploy_contract(
        w3,
        CONTRACTS["TestERC20A"],
        key=KEYS["validator"],
    )
    tx = erc20.functions.transfer(ADDRS["community"], 10).build_transaction(
        {"from": ADDRS["validator"]}
    )
    signed = sign_transaction(w3, tx, KEYS["validator"])
    txhash = w3.eth.send_raw_transaction(signed.raw_transaction)
    exp_gas_used = 51531

    print("wait for prunning happens")
    wait_for_new_blocks(cronos.cosmos_cli(0), 10)

    print("wait for transaction receipt", txhash.hex())
    txreceipt = w3.eth.wait_for_transaction_receipt(txhash)
    assert txreceipt.gasUsed == exp_gas_used
    assert len(txreceipt.logs) == 1
    data = "0x000000000000000000000000000000000000000000000000000000000000000a"
    expect_log = {
        "address": erc20.address,
        "topics": [
            HexBytes(
                abi.event_signature_to_log_topic("Transfer(address,address,uint256)")
            ),
            HexBytes(b"\x00" * 12 + HexBytes(ADDRS["validator"])),
            HexBytes(b"\x00" * 12 + HexBytes(ADDRS["community"])),
        ],
        "data": HexBytes(data),
        "transactionIndex": 0,
        "logIndex": 0,
        "removed": False,
    }
    assert expect_log.items() <= txreceipt.logs[0].items()

    # check get_balance and eth_call don't work on pruned state
    with pytest.raises(Exception):
        w3.eth.get_balance(ADDRS["validator"], block_identifier=txreceipt.blockNumber)
    with pytest.raises(Exception):
        erc20.caller(block_identifier=txreceipt.blockNumber).balanceOf(
            ADDRS["validator"]
        )

    # check block bloom
    block = w3.eth.get_block(txreceipt.blockNumber)
    assert "baseFeePerGas" in block
    assert block.miner == "0x0000000000000000000000000000000000000000"
    bloom = BloomFilter(big_endian_to_int(block.logsBloom))
    assert HexBytes(erc20.address) in bloom
    for topic in expect_log["topics"]:
        assert topic in bloom

    tx1 = w3.eth.get_transaction(txhash)
    tx2 = w3.eth.get_transaction_by_block(
        txreceipt.blockNumber, txreceipt.transactionIndex
    )
    exp_tx = AttributeDict(
        {
            "from": "0x57f96e6B86CdeFdB3d412547816a82E3E0EbF9D2",
            "gas": exp_gas_used,
            "input": HexBytes(
                "0xa9059cbb000000000000000000000000378c50d9264c63f3f92b806d4ee56e"
                "9d86ffb3ec000000000000000000000000000000000000000000000000000000"
                "000000000a"
            ),
            "nonce": 2,
            "to": erc20.address,
            "transactionIndex": 0,
            "value": 0,
            "type": 2,
            "accessList": [],
            "chainId": 777,
        }
    )
    assert tx1 == tx2
    for name in exp_tx.keys():
        assert tx1[name] == tx2[name] == exp_tx[name]

    print(
        w3.eth.get_logs(
            {"fromBlock": txreceipt.blockNumber, "toBlock": txreceipt.blockNumber}
        )
    )
