from pathlib import Path

import pytest
from eth_bloom import BloomFilter
from eth_utils import abi, big_endian_to_int
from hexbytes import HexBytes

from .utils import ADDRS, KEYS, deploy_contract, send_transaction, wait_for_block, wait_for_port, Greeter
from pystarport import cluster
import web3

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
    txhash = contract.functions.test_native_transfer(amount).transact(
        {"from": w3.eth.coinbase}
    )
    receipt = w3.eth.wait_for_transaction_receipt(txhash)
    assert receipt.status == 0, "should fail"

def test_statesync(cronos):
    ## cronos fixture
    # Load cronos-devnet.yaml
    # Spawn pystarport with the yaml, port 26650 (multiple nodes will be created based on `validators`)
    # Return a Cronos object (Defined in network.py)
    w3 = cronos.w3

    ## do some transactions
    # DEPRECATED: Do a tx bank transaction
    # from_addr = "crc1q04jewhxw4xxu3vlg3rc85240h9q7ns6hglz0g"
    # to_addr = "crc16z0herz998946wr659lr84c8c556da55dc34hh"
    # coins = "10basetcro"
    # node = cronos.node_rpc(0)
    # txhash_0 = cronos.cosmos_cli(0).transfer(from_addr, to_addr, coins)["txhash"]

    # Do an ethereum transfer
    tx = {"to": ADDRS["community"], "value": 10000, "gasPrice": w3.eth.gas_price}
    txhash_0 = send_transaction(w3, tx, KEYS["validator"])["transactionHash"].hex()

    # Deploy greeter contract
    greeter = Greeter()
    txhash_1 = greeter.deploy(w3)

    # Wait 1 more block
    wait_for_block(cronos.cosmos_cli(0), int(cronos.cosmos_cli(0).status()["SyncInfo"]["latest_block_height"]) + 1)

    # Check the transactions are added
    # assert cronos.cosmos_cli(0).query_tx("hash", txhash_0)["txhash"] == txhash_0 # DEPRECATED
    assert w3.eth.get_transaction(txhash_0) != None
    assert w3.eth.get_transaction(txhash_1) != None

    ## add a new state sync node, sync
    # We can not use the cronos fixture to do statesync, since they are full nodes.
    # We can only create a new node with statesync config
    data = Path(cronos.base_dir).parent # Same data dir as cronos fixture
    chain_id = cronos.config['chain_id'] # Same chain_id as cronos fixture
    cmd = "cronosd"
    # create a clustercli object from ClusterCLI class
    clustercli = cluster.ClusterCLI(data, cmd=cmd, chain_id=chain_id)
    # create a new node with statesync enabled
    i = clustercli.create_node(moniker="statesync", statesync=True)
    clustercli.supervisor.startProcess(f"{clustercli.chain_id}-node{i}")
    # Wait 1 more block
    wait_for_block(clustercli.cosmos_cli(i), int(cronos.cosmos_cli(0).status()["SyncInfo"]["latest_block_height"]) + 1)

    ## check query chain state works
    assert clustercli.status(i)["SyncInfo"]["catching_up"] == False

    ## check query old transaction does't work
    # Get we3 provider
    wait_for_port(8545)
    statesync_w3 = web3.Web3(web3.providers.HTTPProvider("http://localhost:8545"))
    with pytest.raises(web3.exceptions.TransactionNotFound):
        # clustercli.cosmos_cli(i).query_tx("hash", txhash_0) # DEPRECATED
        statesync_w3.eth.get_transaction(txhash_0)

    with pytest.raises(web3.exceptions.TransactionNotFound):
        statesync_w3.eth.get_transaction(txhash_1)

    ## execute new transactions
    # output = clustercli.cosmos_cli(0).transfer(from_addr, to_addr, coins) # this would have problem!
    # txhash_2 = cronos.cosmos_cli(0).transfer(from_addr, to_addr, coins)["txhash"] # DEPRECATED
    txhash_2 = send_transaction(w3, tx, KEYS["validator"])["transactionHash"].hex()
    txhash_3 = greeter.call_contact(w3)
    # Wait 1 more block
    wait_for_block(clustercli.cosmos_cli(i), int(cronos.cosmos_cli(0).status()["SyncInfo"]["latest_block_height"]) + 1)

    ## check query chain state works
    assert clustercli.status(i)["SyncInfo"]["catching_up"] == False

    ## check query new transaction works
    # assert clustercli.cosmos_cli(i).query_tx("hash", txhash_2)["txhash"] == txhash_2 # DEPRECATED
    assert w3.eth.get_transaction(txhash_2) != None
    assert w3.eth.get_transaction(txhash_3) != None

    print("succesfully syncing")
