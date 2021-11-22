import json
import time
from pathlib import Path

import pytest
import subprocess

from .network import setup_chainmain, setup_cronos, setup_hermes
from .utils import wait_for_port


@pytest.fixture(scope="module")
def cronos(request, tmp_path_factory):
    "start-cronos"
    yield from setup_cronos(tmp_path_factory.mktemp("cronos"), 26700, True)


@pytest.fixture(scope="module")
def chainmain(tmp_path_factory):
    # "start-cronos"
    yield from setup_chainmain(tmp_path_factory.mktemp("chainmain"), 26800)


@pytest.fixture(scope="module")
def hermes(tmp_path_factory):
    time.sleep(20)
    yield from setup_hermes(tmp_path_factory.mktemp("hermes"))


def get_balance(chain, addr, denom):
    coins = chain.cosmos_cli(0).balances(addr)
    for coin in coins:
        if coin["denom"] == denom:
            value = int(coin["amount"])
            return value
    return 0


def test_ibc(cronos, chainmain, hermes):
    # wait for hermes
    hermes_rest_port = 3000
    wait_for_port(hermes_rest_port)
    output = subprocess.getoutput(
        f"curl -s -X GET 'http://127.0.0.1:{hermes_rest_port}/state' | jq"
    )
    assert json.loads(output)["status"] == "success"

    my_ibc0 = "chainmain-1"
    my_ibc1 = "cronos_777-1"
    my_channel = "channel-0"
    my_config = hermes.configpath
    # signer21
    coin_receiver = "crc1q04jewhxw4xxu3vlg3rc85240h9q7ns6hglz0g"
    src_amount = 5
    src_denom = "basecro"
    dst_denom = "basetcro"
    # dstchainid srcchainid srcportid srchannelid
    # chainmain-1 -> cronos_777-1
    cmd = f"hermes -c {my_config} tx raw ft-transfer \
    {my_ibc1} {my_ibc0} transfer {my_channel} {src_amount} \
    -o 1000 -n 1 -d {src_denom} -r {coin_receiver} -k testkey"
    _ = subprocess.getoutput(cmd)
    dstaddr = f"{coin_receiver}"
    olddstbalance = get_balance(cronos, dstaddr, dst_denom)
    time.sleep(5)
    newdstbalance = get_balance(cronos, dstaddr, dst_denom)
    expectedbalance = olddstbalance + src_amount * (10 ** 10)
    assert expectedbalance == newdstbalance


def test_ibc_reverse(cronos, chainmain, hermes):
    # wait for hermes
    hermes_rest_port = 3000
    wait_for_port(hermes_rest_port)
    output = subprocess.getoutput(
        f"curl -s -X GET 'http://127.0.0.1:{hermes_rest_port}/state' | jq"
    )
    assert json.loads(output)["status"] == "success"

    # wait for hermes
    my_ibc0 = "chainmain-1"
    my_ibc1 = "cronos_777-1"
    my_channel = "channel-0"
    my_config = hermes.configpath
    # signer21
    coin_receiver = "cro1u08u5dvtnpmlpdq333uj9tcj75yceggszxpnsy"
    src_amount = 2 * (10 ** 10)
    src_denom = "basetcro"
    dst_denom = "ibc/6B5A664BF0AF4F71B2F0BAA33141E2F1321242FBD\
5D19762F541EC971ACB0865"
    # dstchainid srcchainid srcportid srchannelid
    # chainmain-1 <- cronos_777-1
    cmd = f"hermes -c {my_config} tx raw ft-transfer \
    {my_ibc0} {my_ibc1} transfer {my_channel} {src_amount} \
    -o 1000 -n 1 -d {src_denom} -r {coin_receiver} -k testkey"
    _ = subprocess.getoutput(cmd)
    dstaddr = f"{coin_receiver}"
    olddstbalance = get_balance(chainmain, dstaddr, dst_denom)
    time.sleep(5)
    newdstbalance = get_balance(chainmain, dstaddr, dst_denom)
    expectedbalance = olddstbalance + src_amount
    assert expectedbalance == newdstbalance


def test_contract(cronos, chainmain, hermes):
    cronos_chainid = 777
    cronos_gas = 10000000
    cronos_mnemonics = "night renew tonight dinner shaft scheme \
domain oppose echo summer broccoli agent face guitar surface \
belt veteran siren poem alcohol menu custom crunch index"
    web3api = cronos.w3
    web3api.eth.account.enable_unaudited_hdwallet_features()
    account = web3api.eth.account.from_mnemonic(cronos_mnemonics)
    contract_creator_address = account.address
    web3api.eth.get_balance(contract_creator_address)

    contract_path = (
        Path(__file__).parent / "contracts/artifacts/contracts/Greeter.sol/Greeter.json"
    )
    with open(contract_path) as f:
        json_data = f.read()
        contract_json = json.loads(json_data)

    # precompiled contract
    bytecode = contract_json["bytecode"]
    abi = contract_json["abi"]

    web3api.eth.default_account = account
    # deploy
    greeter_contract_class = web3api.eth.contract(abi=abi, bytecode=bytecode)
    nonce = web3api.eth.get_transaction_count(account.address)
    info = {
        "from": account.address,
        "nonce": nonce,
        "gas": cronos_gas,
        "chainId": cronos_chainid,
    }
    txhash = greeter_contract_class.constructor().transact(info)
    txreceipt = web3api.eth.wait_for_transaction_receipt(txhash)

    # call contract
    greeter_contract_instance = web3api.eth.contract(
        address=txreceipt.contractAddress, abi=abi
    )
    greeter_call_result = greeter_contract_instance.functions.greet().call(info)

    # change
    nonce = web3api.eth.get_transaction_count(account.address)
    info["nonce"] = nonce
    txhash = greeter_contract_instance.functions.setGreeting("world").transact(info)
    web3api.eth.wait_for_transaction_receipt(txhash)

    # call contract
    greeter_call_result = greeter_contract_instance.functions.greet().call(info)
    assert "world" == greeter_call_result
