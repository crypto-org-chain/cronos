import concurrent.futures
import json
import tempfile
import time
from pathlib import Path

import pytest
import web3
from eth_bloom import BloomFilter
from eth_utils import abi, big_endian_to_int
from hexbytes import HexBytes
from pystarport import cluster, ports

from .utils import (
    ADDRS,
    CONTRACTS,
    KEYS,
    Greeter,
    RevertTestContract,
    contract_address,
    contract_path,
    deploy_contract,
    modify_command_in_supervisor_config,
    send_transaction,
    sign_transaction,
    supervisorctl,
    wait_for_block,
    wait_for_port,
)


def test_basic(cluster):
    w3 = cluster.w3
    assert w3.eth.chain_id == 777


def test_events(cluster, suspend_capture):
    w3 = cluster.w3
    erc20 = deploy_contract(
        w3,
        CONTRACTS["TestERC20A"],
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
        CONTRACTS["TestERC20A"],
    )

    amount = 100

    tx = contract.functions.test_native_transfer(amount).buildTransaction(
        {"from": ADDRS["validator"]}
    )
    receipt = send_transaction(w3, tx)

    # expect failure, because contract is not connected with native denom yet
    # TODO complete the test after gov managed token mapping is implemented.
    assert receipt.status == 0


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
    initial_balance = w3.eth.get_balance(ADDRS["community"])
    tx = {"to": ADDRS["community"], "value": tx_value, "gasPrice": gas_price}
    txhash_0 = send_transaction(w3, tx, KEYS["validator"])["transactionHash"].hex()

    # Deploy greeter contract
    greeter = Greeter(
        CONTRACTS["Greeter"],
        KEYS["validator"],
    )
    txhash_1 = greeter.deploy(w3)["transactionHash"].hex()

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
    txhash_3 = greeter.transfer("world")["transactionHash"].hex()
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
    gas_price = w3.eth.gas_price

    # send transaction
    txhash_1 = send_transaction(
        w3,
        {"to": ADDRS["community"], "value": 10000, "gasPrice": gas_price},
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
                "gasPrice": gas_price,
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
        "test_revert_1": RevertTestContract(
            CONTRACTS["TestRevert"],
            KEYS["validator"],
        ),
        "test_revert_2": RevertTestContract(
            CONTRACTS["TestRevert"],
            KEYS["community"],
        ),
        "greeter_1": Greeter(
            CONTRACTS["Greeter"],
            KEYS["signer1"],
        ),
        "greeter_2": Greeter(
            CONTRACTS["Greeter"],
            KEYS["signer2"],
        ),
    }

    with concurrent.futures.ThreadPoolExecutor(4) as executor:
        future_to_contract = {
            executor.submit(contract.deploy, w3): name
            for name, contract in contracts.items()
        }

        assert_receipt_transaction_and_block(w3, future_to_contract)

    # Do Multiple contract calls
    with concurrent.futures.ThreadPoolExecutor(4) as executor:
        futures = []
        futures.append(
            executor.submit(contracts["test_revert_1"].transfer, 5 * (10 ** 18) - 1)
        )
        futures.append(
            executor.submit(contracts["test_revert_2"].transfer, 5 * (10 ** 18))
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
        data = future.result()
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
    # print(block)

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


def test_exception(cluster):
    w3 = cluster.w3
    contract = deploy_contract(
        w3,
        CONTRACTS["TestRevert"],
    )
    with pytest.raises(web3.exceptions.ContractLogicError):
        send_transaction(
            w3, contract.functions.transfer(5 * (10 ** 18) - 1).buildTransaction()
        )
    assert 0 == contract.caller.query()

    receipt = send_transaction(
        w3, contract.functions.transfer(5 * (10 ** 18)).buildTransaction()
    )
    assert receipt.status == 1, "should be succesfully"
    assert 5 * (10 ** 18) == contract.caller.query()


def test_message_call(cronos):
    "stress test the evm by doing message calls as much as possible"
    w3 = cronos.w3
    contract = deploy_contract(
        w3,
        CONTRACTS["TestMessageCall"],
        key=KEYS["community"],
    )
    iterations = 13000
    tx = contract.functions.test(iterations).buildTransaction()

    begin = time.time()
    tx["gas"] = w3.eth.estimate_gas(tx)
    assert time.time() - begin < 5  # should finish in reasonable time

    receipt = send_transaction(w3, tx, KEYS["community"])
    assert 23828976 == receipt.cumulativeGasUsed
    assert receipt.status == 1, "shouldn't fail"
    assert len(receipt.logs) == iterations


def test_suicide(cluster):
    """
    test compliance of contract suicide
    - within the tx, after contract suicide, the code is still available.
    - after the tx, the code is not available.
    """
    w3 = cluster.w3
    destroyee = deploy_contract(
        w3,
        contract_path("Destroyee", "TestSuicide.sol"),
    )
    destroyer = deploy_contract(
        w3,
        contract_path("Destroyer", "TestSuicide.sol"),
    )
    assert len(w3.eth.get_code(destroyee.address)) > 0
    assert len(w3.eth.get_code(destroyer.address)) > 0

    tx = destroyer.functions.check_codesize_after_suicide(
        destroyee.address
    ).buildTransaction()
    receipt = send_transaction(w3, tx)
    assert receipt.status == 1

    assert len(w3.eth.get_code(destroyee.address)) == 0


def test_batch_tx(cronos):
    "send multiple eth txs in single cosmos tx"
    w3 = cronos.w3
    sender = ADDRS["validator"]
    recipient = ADDRS["community"]
    nonce = w3.eth.get_transaction_count(sender)
    info = json.load(open(CONTRACTS["TestERC20Utility"]))
    contract = w3.eth.contract(abi=info["abi"], bytecode=info["bytecode"])
    deploy_tx = contract.constructor().buildTransaction(
        {"from": sender, "nonce": nonce}
    )
    contract = w3.eth.contract(address=contract_address(sender, nonce), abi=info["abi"])
    transfer_tx1 = contract.functions.transfer(recipient, 1000).buildTransaction(
        {"from": sender, "nonce": nonce + 1, "gas": 200000}
    )
    transfer_tx2 = contract.functions.transfer(recipient, 1000).buildTransaction(
        {"from": sender, "nonce": nonce + 2, "gas": 200000}
    )

    signed_txs = [
        sign_transaction(w3, deploy_tx, KEYS["validator"]),
        sign_transaction(w3, transfer_tx1, KEYS["validator"]),
        sign_transaction(w3, transfer_tx2, KEYS["validator"]),
    ]
    tmp_txs = [
        cronos.cosmos_cli().build_evm_tx(signed.rawTransaction.hex())
        for signed in signed_txs
    ]

    msgs = [tx["body"]["messages"][0] for tx in tmp_txs]
    fee = sum(int(tx["auth_info"]["fee"]["amount"][0]["amount"]) for tx in tmp_txs)
    gas_limit = sum(int(tx["auth_info"]["fee"]["gas_limit"]) for tx in tmp_txs)

    # build batch cosmos tx
    cosmos_tx = {
        "body": {
            "messages": msgs,
            "memo": "",
            "timeout_height": "0",
            "extension_options": [
                {"@type": "/ethermint.evm.v1.ExtensionOptionsEthereumTx"}
            ],
            "non_critical_extension_options": [],
        },
        "auth_info": {
            "signer_infos": [],
            "fee": {
                "amount": [{"denom": "basetcro", "amount": str(fee)}],
                "gas_limit": str(gas_limit),
                "payer": "",
                "granter": "",
            },
        },
        "signatures": [],
    }
    with tempfile.NamedTemporaryFile("w") as fp:
        json.dump(cosmos_tx, fp)
        fp.flush()
        rsp = cronos.cosmos_cli().broadcast_tx(fp.name)
        assert rsp["code"] == 0, rsp["raw_log"]

    receipts = [
        w3.eth.wait_for_transaction_receipt(signed.hash) for signed in signed_txs
    ]

    assert 2000 == contract.caller.balanceOf(recipient)

    # check logs
    assert receipts[0].contractAddress == contract.address

    assert receipts[0].transactionIndex == 0
    assert receipts[1].transactionIndex == 1
    assert receipts[2].transactionIndex == 2

    assert receipts[0].logs[0].logIndex == 0
    assert receipts[1].logs[0].logIndex == 1
    assert receipts[2].logs[0].logIndex == 2

    assert receipts[0].cumulativeGasUsed == receipts[0].gasUsed
    assert receipts[1].cumulativeGasUsed == receipts[0].gasUsed + receipts[1].gasUsed
    assert (
        receipts[2].cumulativeGasUsed
        == receipts[0].gasUsed + receipts[1].gasUsed + receipts[2].gasUsed
    )

    # check traceTransaction
    rsps = [
        w3.provider.make_request("debug_traceTransaction", [signed.hash.hex()])[
            "result"
        ]
        for signed in signed_txs
    ]

    for rsp, receipt in zip(rsps, receipts):
        assert not rsp["failed"]
        assert receipt.gasUsed == rsp["gas"]

    # check get_transaction_by_block
    txs = [
        w3.eth.get_transaction_by_block(receipts[0].blockNumber, i) for i in range(3)
    ]
    for tx, signed in zip(txs, signed_txs):
        assert tx.hash == signed.hash


def test_log0(cluster):
    """
    test compliance of empty topics behavior
    """
    w3 = cluster.w3
    contract = deploy_contract(
        w3,
        Path(__file__).parent
        / "contracts/artifacts/contracts/TestERC20Utility.sol/TestERC20Utility.json",
    )
    tx = contract.functions.test_log0().buildTransaction({"from": ADDRS["validator"]})
    receipt = send_transaction(w3, tx, KEYS["validator"])
    assert len(receipt.logs) == 1
    log = receipt.logs[0]
    assert log.topics == []
    assert (
        log.data == "0x68656c6c6f20776f726c64000000000000000000000000000000000000000000"
    )


def test_contract(cronos):
    "test Greeter contract"
    w3 = cronos.w3
    contract = deploy_contract(w3, CONTRACTS["Greeter"])
    assert "Hello" == contract.caller.greet()

    # change
    tx = contract.functions.setGreeting("world").buildTransaction()
    receipt = send_transaction(w3, tx)
    assert receipt.status == 1

    # call contract
    greeter_call_result = contract.caller.greet()
    assert "world" == greeter_call_result


@pytest.mark.parametrize("max_gas_wanted", [80000000, 40000000, 25000000, 500000])
def test_tx_inclusion(cronos, max_gas_wanted):
    """
    - send multiple heavy transactions at the same time.
    - check they are included in consecutively blocks without failure.

    test against different max-gas-wanted configuration.
    """
    modify_command_in_supervisor_config(
        cronos.base_dir / "tasks.ini",
        lambda cmd: f"{cmd} --evm.max-tx-gas-wanted {max_gas_wanted}",
    )
    supervisorctl(cronos.base_dir / "../tasks.ini", "update")
    wait_for_port(ports.evmrpc_port(cronos.base_port(0)))

    w3 = cronos.w3
    block_gas_limit = 81500000
    tx_gas_limit = 80000000
    max_tx_in_block = block_gas_limit // min(max_gas_wanted, tx_gas_limit)
    print("max_tx_in_block", max_tx_in_block)
    amount = 1000
    # use different sender accounts to be able be send concurrently
    signed_txs = []
    for account in ["validator", "community", "signer1", "signer2"]:
        signed_txs.append(
            sign_transaction(
                w3,
                {
                    "to": ADDRS["validator"],
                    "value": amount,
                    "gas": tx_gas_limit,
                },
                KEYS[account],
            )
        )

    for signed in signed_txs:
        w3.eth.send_raw_transaction(signed.rawTransaction)

    receipts = [
        w3.eth.wait_for_transaction_receipt(signed.hash) for signed in signed_txs
    ]

    # the transactions should be included according to max_gas_wanted
    if max_tx_in_block == 1:
        for receipt, next_receipt in zip(receipts, receipts[1:]):
            assert next_receipt.blockNumber == receipt.blockNumber + 1
    elif max_tx_in_block == 2:
        assert receipts[0].blockNumber == receipts[1].blockNumber
        assert (
            receipts[2].blockNumber
            == receipts[3].blockNumber
            == receipts[0].blockNumber + 1
        )
    elif max_tx_in_block == 3:
        assert (
            receipts[0].blockNumber
            == receipts[1].blockNumber
            == receipts[2].blockNumber
        )
        assert receipts[3].blockNumber == receipts[0].blockNumber + 1
    else:
        assert (
            receipts[0].blockNumber
            == receipts[1].blockNumber
            == receipts[2].blockNumber
            == receipts[3].blockNumber
        )
