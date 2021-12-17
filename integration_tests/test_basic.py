import concurrent.futures
from pathlib import Path

import pytest
import web3
from eth_bloom import BloomFilter
from eth_utils import abi, big_endian_to_int
from hexbytes import HexBytes
from pystarport import cluster, ports

from .utils import (
    ADDRS,
    KEYS,
    Greeter,
    TestRevert,
    deploy_contract,
    send_transaction,
    wait_for_block,
    wait_for_current_block_confirmation,
    wait_for_port,
)


def test_basic(cluster):
    w3 = cluster.w3
    assert w3.eth.chain_id == 777
    assert w3.eth.get_balance(ADDRS["community"]) == 10000000000000000000000


def test_events(cluster, suspend_capture):
    w3 = cluster.w3
    erc20 = deploy_contract(
        w3,
        Path(__file__).parent
        / "contracts/artifacts/contracts/TestERC20A.sol/TestERC20A.json",
        key=KEYS["validator"],
    )
    tx = erc20.functions.transfer(ADDRS["community"], 10).buildTransaction(
        {"from": ADDRS["validator"]}
    )
    txreceipt = send_transaction(w3, tx, KEYS["validator"])
    assert len(txreceipt.logs) == 1
    expect_log = {
        "address": erc20.address,
        "topics": [
            HexBytes(
                abi.event_signature_to_log_topic("Transfer(address,address,uint256)")
            ),
            HexBytes(b"\x00" * 12 + HexBytes(ADDRS["validator"])),
            HexBytes(b"\x00" * 12 + HexBytes(ADDRS["community"])),
        ],
        "data": "0x000000000000000000000000000000000000000000000000000000000000000a",
        "transactionIndex": 0,
        "logIndex": 0,
        "removed": False,
    }
    assert expect_log.items() <= txreceipt.logs[0].items()

    # check block bloom
    bloom = BloomFilter(
        big_endian_to_int(w3.eth.get_block(txreceipt.blockNumber).logsBloom)
    )
    assert HexBytes(erc20.address) in bloom
    for topic in expect_log["topics"]:
        assert topic in bloom


def test_minimal_gas_price(cronos):
    w3 = cronos.w3
    gas_price = w3.eth.gas_price
    assert gas_price == 5000000000000
    tx = {
        "to": "0x0000000000000000000000000000000000000000",
        "value": 10000,
    }
    with pytest.raises(ValueError):
        send_transaction(
            w3,
            {**tx, "gasPrice": 1},
            KEYS["community"],
        )
    receipt = send_transaction(
        w3,
        {**tx, "gasPrice": gas_price},
        KEYS["validator"],
    )
    assert receipt.status == 1


def test_native_call(cronos):
    """
    test contract native call on cronos network
    - deploy test contract
    - run native call, expect failure, becuase no native fund in contract
    - send native tokens to contract account
    - run again, expect success and check balance
    """
    w3 = cronos.w3
    contract = deploy_contract(
        w3,
        Path(__file__).parent
        / "contracts/artifacts/contracts/TestERC20A.sol/TestERC20A.json",
    )

    amount = 100

    # the coinbase in web3 api is the same address as the validator account in
    # cosmos api

    # expect failure, because contract is not connected with native denom yet
    # TODO complete the test after gov managed token mapping is implemented.
    with pytest.raises(ValueError):
        contract.functions.test_native_transfer(amount).transact(
            {"from": w3.eth.coinbase}
        )


def test_statesync(cronos):
    # cronos fixture
    # Load cronos-devnet.yaml
    # Spawn pystarport with the yaml, port 26650
    # (multiple nodes will be created based on `validators`)
    # Return a Cronos object (Defined in network.py)
    w3 = cronos.w3

    # do some transactions
    # DEPRECATED: Do a tx bank transaction
    # from_addr = "crc1q04jewhxw4xxu3vlg3rc85240h9q7ns6hglz0g"
    # to_addr = "crc16z0herz998946wr659lr84c8c556da55dc34hh"
    # coins = "10basetcro"
    # node = cronos.node_rpc(0)
    # txhash_0 = cronos.cosmos_cli(0).transfer(from_addr, to_addr, coins)["txhash"]

    # Do an ethereum transfer
    tx_value = 10000
    gas_price = w3.eth.gas_price
    initial_balance = 10000000000000000000000
    tx = {"to": ADDRS["community"], "value": tx_value, "gasPrice": gas_price}
    txhash_0 = send_transaction(w3, tx, KEYS["validator"])["transactionHash"].hex()

    # Deploy greeter contract
    greeter = Greeter(
        Path(__file__).parent
        / "contracts/artifacts/contracts/Greeter.sol/Greeter.json",
        KEYS["validator"],
    )
    txhash_1 = greeter.deploy(w3)

    assert w3.eth.get_balance(ADDRS["community"]) == initial_balance + tx_value

    # Wait 5 more block (sometimes not enough blocks can not work)
    wait_for_block(
        cronos.cosmos_cli(0),
        int(cronos.cosmos_cli(0).status()["SyncInfo"]["latest_block_height"]) + 5,
    )

    # Check the transactions are added
    assert w3.eth.get_transaction(txhash_0) is not None
    assert w3.eth.get_transaction(txhash_1) is not None

    # add a new state sync node, sync
    # We can not use the cronos fixture to do statesync, since they are full nodes.
    # We can only create a new node with statesync config
    data = Path(cronos.base_dir).parent  # Same data dir as cronos fixture
    chain_id = cronos.config["chain_id"]  # Same chain_id as cronos fixture
    cmd = "cronosd"
    # create a clustercli object from ClusterCLI class
    clustercli = cluster.ClusterCLI(data, cmd=cmd, chain_id=chain_id)
    # create a new node with statesync enabled
    i = clustercli.create_node(moniker="statesync", statesync=True)
    # Modify the json-rpc addresses to avoid conflict
    cluster.edit_app_cfg(
        clustercli.home(i) / "config/app.toml",
        clustercli.base_port(i),
        {
            "json-rpc": {
                "address": "0.0.0.0:{EVMRPC_PORT}",
                "ws-address": "0.0.0.0:{EVMRPC_PORT_WS}",
            }
        },
    )
    clustercli.supervisor.startProcess(f"{clustercli.chain_id}-node{i}")
    # Wait 1 more block
    wait_for_block(
        clustercli.cosmos_cli(i),
        int(cronos.cosmos_cli(0).status()["SyncInfo"]["latest_block_height"]) + 1,
    )

    # check query chain state works
    assert not clustercli.status(i)["SyncInfo"]["catching_up"]

    # check query old transaction does't work
    # Get we3 provider
    base_port = ports.evmrpc_port(clustercli.base_port(i))
    print("json-rpc port:", base_port)
    wait_for_port(base_port)
    statesync_w3 = web3.Web3(
        web3.providers.HTTPProvider(f"http://localhost:{base_port}")
    )
    with pytest.raises(web3.exceptions.TransactionNotFound):
        statesync_w3.eth.get_transaction(txhash_0)

    with pytest.raises(web3.exceptions.TransactionNotFound):
        statesync_w3.eth.get_transaction(txhash_1)

    # execute new transactions
    txhash_2 = send_transaction(w3, tx, KEYS["validator"])["transactionHash"].hex()
    txhash_3 = greeter.transfer("world")
    # Wait 1 more block
    wait_for_block(
        clustercli.cosmos_cli(i),
        int(cronos.cosmos_cli(0).status()["SyncInfo"]["latest_block_height"]) + 1,
    )

    # check query chain state works
    assert not clustercli.status(i)["SyncInfo"]["catching_up"]

    # check query new transaction works
    assert statesync_w3.eth.get_transaction(txhash_2) is not None
    assert statesync_w3.eth.get_transaction(txhash_3) is not None
    assert (
        statesync_w3.eth.get_balance(ADDRS["community"])
        == initial_balance + tx_value + tx_value
    )

    print("succesfully syncing")


def test_transaction(cronos):
    w3 = cronos.w3

    # send transaction
    txhash_1 = send_transaction(
        w3,
        {"to": ADDRS["community"], "value": 10000, "gasPrice": w3.eth.gas_price},
        KEYS["validator"],
    )["transactionHash"]
    tx1 = w3.eth.get_transaction(txhash_1)
    assert tx1["transactionIndex"] == 0

    initial_block_number = w3.eth.get_block_number()

    # tx already in mempool
    with pytest.raises(ValueError) as exc:
        send_transaction(
            w3,
            {
                "to": ADDRS["community"],
                "value": 10000,
                "gasPrice": w3.eth.gas_price,
                "nonce": w3.eth.get_transaction_count(ADDRS["validator"]) - 1,
            },
            KEYS["validator"],
        )
    assert "tx already in mempool" in str(exc)

    # invalid sequence
    with pytest.raises(ValueError) as exc:
        send_transaction(
            w3,
            {
                "to": ADDRS["community"],
                "value": 10000,
                "gasPrice": w3.eth.gas_price,
                "nonce": w3.eth.get_transaction_count(ADDRS["validator"]) + 1,
            },
            KEYS["validator"],
        )
    assert "invalid sequence" in str(exc)

    # out of gas
    with pytest.raises(ValueError) as exc:
        send_transaction(
            w3,
            {
                "to": ADDRS["community"],
                "value": 10000,
                "gasPrice": w3.eth.gas_price,
                "gas": 1,
            },
            KEYS["validator"],
        )["transactionHash"]
    assert "out of gas" in str(exc)

    # insufficient fee
    with pytest.raises(ValueError) as exc:
        send_transaction(
            w3,
            {
                "to": ADDRS["community"],
                "value": 10000,
                "gasPrice": 1,
            },
            KEYS["validator"],
        )["transactionHash"]
    assert "insufficient fee" in str(exc)

    # check all failed transactions are not included in blockchain
    assert w3.eth.get_block_number() == initial_block_number

    # Deploy multiple contracts
    contracts = {
        "test_revert_1": TestRevert(
            Path(__file__).parent
            / "contracts/artifacts/contracts/TestRevert.sol/TestRevert.json",
            KEYS["validator"],
        ),
        "test_revert_2": TestRevert(
            Path(__file__).parent
            / "contracts/artifacts/contracts/TestRevert.sol/TestRevert.json",
            KEYS["community"],
        ),
        "greeter_1": Greeter(
            Path(__file__).parent
            / "contracts/artifacts/contracts/Greeter.sol/Greeter.json",
            KEYS["signer1"],
        ),
        "greeter_2": Greeter(
            Path(__file__).parent
            / "contracts/artifacts/contracts/Greeter.sol/Greeter.json",
            KEYS["signer2"],
        ),
    }

    with concurrent.futures.ThreadPoolExecutor(4) as executor:
        future_to_contract = {
            executor.submit(contract.deploy, w3): name
            for name, contract in contracts.items()
        }

        assert_receipt_transaction_and_block(w3, future_to_contract)

    wait_for_current_block_confirmation(w3)

    # Do Multiple contract calls
    with concurrent.futures.ThreadPoolExecutor(4) as executor:
        futures = []
        futures.append(
            executor.submit(contracts["test_revert_1"].transfer, 5 * 10 ** 18 - 1)
        )
        futures.append(
            executor.submit(contracts["test_revert_2"].transfer, 5 * 10 ** 18)
        )
        futures.append(executor.submit(contracts["greeter_1"].transfer, "hello"))
        futures.append(executor.submit(contracts["greeter_2"].transfer, "world"))

        assert_receipt_transaction_and_block(w3, futures)

        # revert transaction
        assert futures[0].result()["status"] == 0
        # normal transaction
        assert futures[1].result()["status"] == 1
        # normal transaction
        assert futures[2].result()["status"] == 1
        # normal transaction
        assert futures[3].result()["status"] == 1


def assert_receipt_transaction_and_block(w3, futures):
    receipts = []
    for future in concurrent.futures.as_completed(futures):
        # name = future_to_contract[future]
        try:
            data = future.result()
        except Exception as exc:
            print("%s" % (exc))
        else:
            receipts.append(data)
    assert len(receipts) == 4
    # print(receipts)

    block_number = w3.eth.get_block_number()
    tx_indexes = [0, 1, 2, 3]
    for receipt in receipts:
        # check in the same block
        assert receipt["blockNumber"] == block_number
        # check transactionIndex
        transaction_index = receipt["transactionIndex"]
        assert transaction_index in tx_indexes
        tx_indexes.remove(transaction_index)

    block = w3.eth.get_block(block_number)
    print(block)

    transactions = [
        w3.eth.get_transaction_by_block(block_number, receipt["transactionIndex"])
        for receipt in receipts
    ]
    assert len(transactions) == 4
    for i, transaction in enumerate(transactions):
        # print(transaction)
        # check in the same block
        assert transaction["blockNumber"] == block_number
        # check transactionIndex
        assert transaction["transactionIndex"] == receipts[i]["transactionIndex"]
        # check hash
        assert transaction["hash"] == receipts[i]["transactionHash"]
        # check transaction in block
        assert transaction["hash"] in block["transactions"]
        # check blockNumber in block
        assert transaction["blockNumber"] == block["number"]
