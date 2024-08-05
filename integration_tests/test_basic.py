import hashlib
import json
import subprocess
import time
from concurrent.futures import ThreadPoolExecutor, as_completed
from pathlib import Path

import pytest
import web3
from eth_bloom import BloomFilter
from eth_utils import abi, big_endian_to_int
from hexbytes import HexBytes
from pystarport import cluster, ports

from .cosmoscli import CosmosCLI
from .network import Geth
from .utils import (
    ADDRS,
    CONTRACTS,
    KEYS,
    Greeter,
    RevertTestContract,
    approve_proposal,
    build_batch_tx,
    contract_address,
    contract_path,
    deploy_contract,
    derive_new_account,
    eth_to_bech32,
    get_receipts_by_block,
    get_sync_info,
    modify_command_in_supervisor_config,
    send_transaction,
    send_txs,
    sign_transaction,
    submit_any_proposal,
    wait_for_block,
    wait_for_new_blocks,
    wait_for_port,
)


def test_ica_enabled(cronos, tmp_path):
    cli = cronos.cosmos_cli()
    p = cli.query_ica_params()
    assert p["controller_enabled"]
    p["controller_enabled"] = False
    proposal = tmp_path / "proposal.json"
    # governance module account as signer
    data = hashlib.sha256("gov".encode()).digest()[:20]
    signer = eth_to_bech32(data)
    type = "/ibc.applications.interchain_accounts.controller.v1.MsgUpdateParams"
    proposal_src = {
        "messages": [
            {
                "@type": type,
                "signer": signer,
                "params": p,
            }
        ],
        "deposit": "1basetcro",
        "title": "title",
        "summary": "summary",
    }
    proposal.write_text(json.dumps(proposal_src))
    rsp = cli.submit_gov_proposal(proposal, from_="community")
    assert rsp["code"] == 0, rsp["raw_log"]
    approve_proposal(cronos, rsp["events"])
    print("check params have been updated now")
    p = cli.query_ica_params()
    assert not p["controller_enabled"]


def test_basic(cluster):
    w3 = cluster.w3
    assert w3.eth.chain_id == 777


def test_send_transaction(cluster):
    "test eth_sendTransaction api"
    w3 = cluster.w3
    # wait 1s to avoid unlock error
    if isinstance(cluster, Geth):
        time.sleep(1)
    txhash = w3.eth.send_transaction(
        {
            "from": ADDRS["validator"],
            "to": ADDRS["community"],
            "value": 1000,
        }
    )
    receipt = w3.eth.wait_for_transaction_receipt(txhash)
    assert receipt.status == 1
    assert receipt.gasUsed == 21000


def test_events(cluster, suspend_capture):
    w3 = cluster.w3
    erc20 = deploy_contract(
        w3,
        CONTRACTS["TestERC20A"],
        key=KEYS["validator"],
        exp_gas_used=641641,
    )
    tx = erc20.functions.transfer(ADDRS["community"], 10).build_transaction(
        {"from": ADDRS["validator"]}
    )
    txreceipt = send_transaction(w3, tx, KEYS["validator"])
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
    - run native call, expect failure, because no native fund in contract
    - send native tokens to contract account
    - run again, expect success and check balance
    """
    w3 = cronos.w3
    contract = deploy_contract(
        w3,
        CONTRACTS["TestERC20A"],
    )

    amount = 100

    tx = contract.functions.test_native_transfer(amount).build_transaction(
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
    cli0 = cronos.cosmos_cli(0)
    wait_for_block(cli0, cli0.block_height() + 5)

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
                "address": "127.0.0.1:{EVMRPC_PORT}",
                "ws-address": "127.0.0.1:{EVMRPC_PORT_WS}",
            },
            "memiavl": {
                "enable": True,
                "zero-copy": True,
                "snapshot-interval": 5,
            },
        },
    )
    clustercli.supervisor.startProcess(f"{clustercli.chain_id}-node{i}")
    # Wait 1 more block
    wait_for_block(clustercli.cosmos_cli(i), cli0.block_height() + 1)
    time.sleep(1)

    # check query chain state works
    assert not get_sync_info(clustercli.status(i))["catching_up"]

    # check query old transaction doesn't work
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
    wait_for_block(clustercli.cosmos_cli(i), cli0.block_height() + 1)

    # check query chain state works
    assert not get_sync_info(clustercli.status(i))["catching_up"]

    # check query new transaction works
    assert statesync_w3.eth.get_transaction(txhash_2) is not None
    assert statesync_w3.eth.get_transaction(txhash_3) is not None
    assert (
        statesync_w3.eth.get_balance(ADDRS["community"])
        == initial_balance + tx_value + tx_value
    )

    print("successfully syncing")
    clustercli.supervisor.stopProcess(f"{clustercli.chain_id}-node{i}")


def test_local_statesync(cronos, tmp_path_factory):
    """
    - init a new node, enable versiondb
    - dump snapshot on node0
    - load snapshot to the new node
    - restore the new node state from the snapshot
    - bootstrap cometbft state
    - restore the versiondb from the snapshot
    - startup the node, should sync
    - cleanup
    """
    # wait for the network to grow a little bit
    cli0 = cronos.cosmos_cli(0)
    wait_for_block(cli0, 6)

    sync_info = get_sync_info(cli0.status())
    cronos.supervisorctl("stop", "cronos_777-1-node0")
    tarball = cli0.data_dir / "snapshot.tar.gz"
    height = int(sync_info["latest_block_height"])
    # round down to multplies of memiavl.snapshot-interval
    height -= height % 5

    if height not in set(item.height for item in cli0.list_snapshot()):
        cli0.export_snapshot(height)

    cli0.dump_snapshot(height, tarball)
    cronos.supervisorctl("start", "cronos_777-1-node0")
    wait_for_port(ports.evmrpc_port(cronos.base_port(0)))

    home = tmp_path_factory.mktemp("local_statesync")
    print("home", home)

    i = len(cronos.config["validators"])
    base_port = 26650 + i * 10
    node_rpc = "tcp://127.0.0.1:%d" % ports.rpc_port(base_port)
    cli = CosmosCLI.init(
        "local_statesync",
        Path(home),
        node_rpc,
        cronos.chain_binary,
        "cronos_777-1",
    )

    # init the configs
    peers = ",".join(
        [
            "tcp://%s@%s:%d"
            % (
                cronos.cosmos_cli(i).node_id(),
                val["hostname"],
                ports.p2p_port(val["base_port"]),
            )
            for i, val in enumerate(cronos.config["validators"])
        ]
    )
    rpc_servers = ",".join(cronos.node_rpc(i) for i in range(2))
    trust_height = int(sync_info["latest_block_height"])
    trust_hash = sync_info["latest_block_hash"]

    cluster.edit_tm_cfg(
        Path(home) / "config/config.toml",
        base_port,
        peers,
        {
            "statesync": {
                "rpc_servers": rpc_servers,
                "trust_height": trust_height,
                "trust_hash": trust_hash,
            },
        },
    )
    cluster.edit_app_cfg(
        Path(home) / "config/app.toml",
        base_port,
        {
            "versiondb": {
                "enable": True,
            },
        },
    )

    # restore the states
    cli.load_snapshot(tarball)
    print(cli.list_snapshot())
    cli.restore_snapshot(height)
    cli.bootstrap_state()
    cli.restore_versiondb(height)

    with subprocess.Popen(
        [cronos.chain_binary, "start", "--home", home],
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
    ):
        wait_for_port(ports.rpc_port(base_port))
        # check the node sync normally
        wait_for_new_blocks(cli, 2)
        # check grpc works
        print("distribution", cli.distribution_community(height=height))
        with pytest.raises(Exception) as exc_info:
            cli.distribution_community(height=height - 1)

        assert "collections: not found" in exc_info.value.args[0]


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

    with ThreadPoolExecutor(4) as executor:
        future_to_contract = {
            executor.submit(contract.deploy, w3): name
            for name, contract in contracts.items()
        }

        assert_receipt_transaction_and_block(w3, future_to_contract)

    # Do Multiple contract calls
    with ThreadPoolExecutor(4) as executor:
        futures = []
        futures.append(
            executor.submit(contracts["test_revert_1"].transfer, 5 * (10**18) - 1)
        )
        futures.append(
            executor.submit(contracts["test_revert_2"].transfer, 5 * (10**18))
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
    for future in as_completed(futures):
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
            w3, contract.functions.transfer(5 * (10**18) - 1).build_transaction()
        )
    assert 0 == contract.caller.query()

    receipt = send_transaction(
        w3, contract.functions.transfer(5 * (10**18)).build_transaction()
    )
    assert receipt.status == 1, "should be successfully"
    assert 5 * (10**18) == contract.caller.query()


def test_refund_unused_gas_when_contract_tx_reverted(cluster):
    """
    Call a smart contract method that reverts with very high gas limit

    Call tx receipt should be status 0 (fail)
    Fee is gasUsed * effectiveGasPrice
    """
    w3 = cluster.w3
    contract = deploy_contract(w3, CONTRACTS["TestRevert"])
    more_than_enough_gas = 1000000

    balance_bef = w3.eth.get_balance(ADDRS["community"])
    receipt = send_transaction(
        w3,
        contract.functions.transfer(5 * (10**18) - 1).build_transaction(
            {"gas": more_than_enough_gas}
        ),
        key=KEYS["community"],
    )
    balance_aft = w3.eth.get_balance(ADDRS["community"])

    assert receipt["status"] == 0, "should be a failed tx"
    assert receipt["gasUsed"] != more_than_enough_gas
    assert (
        balance_bef - balance_aft == receipt["gasUsed"] * receipt["effectiveGasPrice"]
    )


def test_message_call(cronos):
    "stress test the evm by doing message calls as much as possible"
    w3 = cronos.w3
    contract = deploy_contract(
        w3,
        CONTRACTS["TestMessageCall"],
        key=KEYS["community"],
    )
    iterations = 13000
    tx = contract.functions.test(iterations).build_transaction()

    begin = time.time()
    tx["gas"] = w3.eth.estimate_gas(tx)
    elapsed = time.time() - begin
    print("elapsed:", elapsed)
    assert elapsed < 5  # should finish in reasonable time

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
    ).build_transaction()
    receipt = send_transaction(w3, tx)
    assert receipt.status == 1

    assert len(w3.eth.get_code(destroyee.address)) == 0


def test_batch_tx(cronos):
    "send multiple eth txs in single cosmos tx"
    w3 = cronos.w3
    cli = cronos.cosmos_cli()
    sender = ADDRS["validator"]
    recipient = ADDRS["community"]
    nonce = w3.eth.get_transaction_count(sender)
    info = json.loads(CONTRACTS["TestERC20Utility"].read_text())
    contract = w3.eth.contract(abi=info["abi"], bytecode=info["bytecode"])
    deploy_tx = contract.constructor().build_transaction(
        {"from": sender, "nonce": nonce}
    )
    contract = w3.eth.contract(address=contract_address(sender, nonce), abi=info["abi"])
    transfer_tx1 = contract.functions.transfer(recipient, 1000).build_transaction(
        {"from": sender, "nonce": nonce + 1, "gas": 200000}
    )
    transfer_tx2 = contract.functions.transfer(recipient, 1000).build_transaction(
        {"from": sender, "nonce": nonce + 2, "gas": 200000}
    )

    cosmos_tx, tx_hashes = build_batch_tx(
        w3, cli, [deploy_tx, transfer_tx1, transfer_tx2]
    )
    rsp = cli.broadcast_tx_json(cosmos_tx)
    assert rsp["code"] == 0, rsp["raw_log"]

    receipts = [w3.eth.wait_for_transaction_receipt(h) for h in tx_hashes]

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
        w3.provider.make_request("debug_traceTransaction", [h.hex()])["result"]
        for h in tx_hashes
    ]

    for rsp, receipt in zip(rsps, receipts):
        assert not rsp["failed"]
        assert receipt.gasUsed == rsp["gas"]

    # check get_transaction_by_block
    txs = [
        w3.eth.get_transaction_by_block(receipts[0].blockNumber, i) for i in range(3)
    ]
    for tx, h in zip(txs, tx_hashes):
        assert tx.hash == h

    # check getBlock
    txs = w3.eth.get_block(receipts[0].blockNumber, True).transactions
    for i in range(3):
        assert txs[i].transactionIndex == i


def test_failed_transfer_tx(cronos):
    """
    It's possible to include a failed transfer transaction in batch tx
    """
    w3 = cronos.w3
    cli = cronos.cosmos_cli()
    sender = ADDRS["community"]
    recipient = ADDRS["validator"]
    nonce = w3.eth.get_transaction_count(sender)
    half_balance = w3.eth.get_balance(sender) // 3 + 1

    # build batch tx, the third tx will fail, but will be included in block
    # because of the batch tx.
    transfer1 = {"from": sender, "nonce": nonce, "to": recipient, "value": half_balance}
    transfer2 = {
        "from": sender,
        "nonce": nonce + 1,
        "to": recipient,
        "value": half_balance,
    }
    transfer3 = {
        "from": sender,
        "nonce": nonce + 2,
        "to": recipient,
        "value": half_balance,
    }
    cosmos_tx, tx_hashes = build_batch_tx(
        w3, cli, [transfer1, transfer2, transfer3], KEYS["community"]
    )
    rsp = cli.broadcast_tx_json(cosmos_tx)
    assert rsp["code"] == 0, rsp["raw_log"]

    receipts = [w3.eth.wait_for_transaction_receipt(h) for h in tx_hashes]
    assert receipts[0].status == receipts[1].status == 1
    assert receipts[2].status == 0

    # test the cronos_getTransactionReceiptsByBlock api
    rsp = get_receipts_by_block(w3, receipts[0].blockNumber)
    assert "error" not in rsp, rsp["error"]
    assert len(receipts) == len(rsp["result"])
    for a, b in zip(receipts, rsp["result"]):
        assert a == b

    # check traceTransaction
    rsps = [
        w3.provider.make_request("debug_traceTransaction", [h.hex()]) for h in tx_hashes
    ]
    for rsp, receipt in zip(rsps, receipts):
        if receipt.status == 1:
            result = rsp["result"]
            assert not result["failed"]
            assert receipt.gasUsed == result["gas"]
        else:
            assert rsp["error"] == {
                "code": -32000,
                "message": (
                    "rpc error: code = Internal desc = "
                    "insufficient balance for transfer"
                ),
            }


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
    tx = contract.functions.test_log0().build_transaction({"from": ADDRS["validator"]})
    receipt = send_transaction(w3, tx, KEYS["validator"])
    assert len(receipt.logs) == 1
    log = receipt.logs[0]
    assert log.topics == []
    data = "0x68656c6c6f20776f726c64000000000000000000000000000000000000000000"
    assert log.data == HexBytes(data)


def test_contract(cronos):
    "test Greeter contract"
    w3 = cronos.w3
    contract = deploy_contract(w3, CONTRACTS["Greeter"])
    assert "Hello" == contract.caller.greet()

    # change
    tx = contract.functions.setGreeting("world").build_transaction()
    receipt = send_transaction(w3, tx)
    assert receipt.status == 1

    # call contract
    greeter_call_result = contract.caller.greet()
    assert "world" == greeter_call_result


origin_cmd = None


@pytest.mark.unmarked
@pytest.mark.parametrize("max_gas_wanted", [80000000, 40000000, 25000000, 500000, None])
def test_tx_inclusion(cronos, max_gas_wanted):
    """
    - send multiple heavy transactions at the same time.
    - check they are included in consecutively blocks without failure.

    test against different max-gas-wanted configuration.
    """

    def fn(cmd):
        global origin_cmd
        if origin_cmd is None:
            origin_cmd = cmd
        if max_gas_wanted is None:
            return origin_cmd
        return f"{origin_cmd} --evm.max-tx-gas-wanted {max_gas_wanted}"

    modify_command_in_supervisor_config(
        cronos.base_dir / "tasks.ini",
        lambda cmd: fn(cmd),
    )
    cronos.supervisorctl("update")
    wait_for_port(ports.evmrpc_port(cronos.base_port(0)))

    # reset to origin_cmd only
    if max_gas_wanted is None:
        return

    w3 = cronos.w3
    cli = cronos.cosmos_cli()
    block_gas_limit = 81500000
    tx_gas_limit = 80000000
    max_tx_in_block = block_gas_limit // min(max_gas_wanted, tx_gas_limit)
    print("max_tx_in_block", max_tx_in_block)
    to = ADDRS["validator"]
    params = {"gas": tx_gas_limit}
    _, sended_hash_set = send_txs(w3, cli, to, list(KEYS.values())[0:4], params)
    block_nums = [
        w3.eth.wait_for_transaction_receipt(h).blockNumber for h in sended_hash_set
    ]
    block_nums.sort()
    print(f"all block numbers: {block_nums}")
    # the transactions should be included according to max_gas_wanted
    if max_tx_in_block == 1:
        for block_num, next_block_num in zip(block_nums, block_nums[1:]):
            assert next_block_num == block_num + 1 or next_block_num == block_num + 2
    else:
        for num in block_nums[1:max_tx_in_block]:
            assert num == block_nums[0]
        for num in block_nums[max_tx_in_block:]:
            assert num == block_nums[0] + 1 or num == block_nums[0] + 2


def test_replay_protection(cronos):
    w3 = cronos.w3
    # https://etherscan.io/tx/0x06d2fa464546e99d2147e1fc997ddb624cec9c8c5e25a050cc381ee8a384eed3
    raw = (
        (
            Path(__file__).parent / "configs/replay-tx-0x"
            "06d2fa464546e99d2147e1fc997ddb62"
            "4cec9c8c5e25a050cc381ee8a384eed3.tx"
        )
        .read_text()
        .strip()
    )
    with pytest.raises(
        Exception,
        match=r"only replay-protected \(EIP-155\) transactions allowed over RPC",
    ):
        w3.eth.send_raw_transaction(HexBytes(raw))


@pytest.mark.gov
def test_submit_any_proposal(cronos, tmp_path):
    submit_any_proposal(cronos, tmp_path)


@pytest.mark.gov
def test_submit_send_enabled(cronos, tmp_path):
    # check bank send enable
    cli = cronos.cosmos_cli()
    denoms = ["basetcro", "stake"]
    assert len(cli.query_bank_send(*denoms)) == 0, "should be empty"
    proposal = tmp_path / "proposal.json"
    # governance module account as signer
    signer = "crc10d07y265gmmuvt4z0w9aw880jnsr700jdufnyd"
    send_enable = [
        {"denom": "basetcro"},
        {"denom": "stake", "enabled": True},
    ]
    proposal_src = {
        "messages": [
            {
                "@type": "/cosmos.bank.v1beta1.MsgSetSendEnabled",
                "authority": signer,
                "sendEnabled": send_enable,
            }
        ],
        "deposit": "1basetcro",
        "title": "title",
        "summary": "summary",
    }
    proposal.write_text(json.dumps(proposal_src))
    rsp = cli.submit_gov_proposal(proposal, from_="community")
    assert rsp["code"] == 0, rsp["raw_log"]
    approve_proposal(cronos, rsp["events"])
    print("check params have been updated now")
    assert cli.query_bank_send(*denoms) == send_enable


def test_app_hash_mismatch(cronos):
    w3 = cronos.w3
    cli = cronos.cosmos_cli()
    acc = derive_new_account(3)
    sender = acc.address

    # fund new sender
    fund = 3000000000000000000
    tx = {"to": sender, "value": fund, "gasPrice": w3.eth.gas_price}
    send_transaction(w3, tx)
    assert w3.eth.get_balance(sender, "latest") == fund
    nonce = w3.eth.get_transaction_count(sender)
    wait_for_new_blocks(cli, 1)
    txhashes = []
    total = 3
    for n in range(total):
        tx = {
            "to": "0x2956c404227Cc544Ea6c3f4a36702D0FD73d20A2",
            "value": fund // total,
            "gas": 21000,
            "maxFeePerGas": 6556868066901,
            "maxPriorityFeePerGas": 1500000000,
            "nonce": nonce + n,
        }
        signed = sign_transaction(w3, tx, acc.key)
        txhash = w3.eth.send_raw_transaction(signed.rawTransaction)
        txhashes.append(txhash)
    for txhash in txhashes[0 : total - 1]:
        res = w3.eth.wait_for_transaction_receipt(txhash)
        assert res.status == 1
    w3_wait_for_block(w3, w3.eth.block_number + 3, timeout=30)
