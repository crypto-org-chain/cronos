import json
import subprocess
import time

import pytest

from .network import setup_chainmain, setup_cronos, setup_hermes
from .utils import (
    ADDRS,
    CONTRACTS,
    deploy_contract,
    eth_to_bech32,
    send_transaction,
    wait_for_fn,
    wait_for_port,
)


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
    return chain.cosmos_cli().balance(addr, denom)


def test_ibc(cronos, chainmain, hermes):
    "test sending basecro from crypto-org chain to cronos"
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
    # signer2
    coin_receiver = eth_to_bech32(ADDRS["signer2"])
    src_amount = 10
    dst_amount = src_amount * (10**10)  # the decimal places difference
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
    newdstbalance = 0

    def check_balance_change():
        nonlocal newdstbalance
        newdstbalance = get_balance(cronos, dstaddr, dst_denom)
        return newdstbalance != olddstbalance

    wait_for_fn("check balance change", check_balance_change)
    expectedbalance = olddstbalance + dst_amount
    assert expectedbalance == newdstbalance


def test_cronos_transfer_tokens(cronos, chainmain, hermes):
    """
    test sending basetcro from cronos to crypto-org-chain using cli transfer_tokens.
    depends on `test_ibc` to send the original coins.
    """
    # wait for hermes
    hermes_rest_port = 3000
    wait_for_port(hermes_rest_port)
    output = subprocess.getoutput(
        f"curl -s -X GET 'http://127.0.0.1:{hermes_rest_port}/state' | jq"
    )
    assert json.loads(output)["status"] == "success"

    coin_receiver = chainmain.cosmos_cli().address("signer2")
    dst_amount = 2
    src_amount = dst_amount * (10**10)  # the decimal places difference

    # case 1: use cronos cli
    oldbalance = get_balance(chainmain, coin_receiver, "basecro")
    cli = cronos.cosmos_cli()
    rsp = cli.transfer_tokens(
        cli.address("signer2"), coin_receiver, f"{src_amount}basetcro"
    )
    assert rsp["code"] == 0, rsp["raw_log"]

    newbalance = 0

    def check_balance_change():
        nonlocal newbalance
        newbalance = get_balance(chainmain, coin_receiver, "basecro")
        return oldbalance != newbalance

    wait_for_fn("check balance change", check_balance_change)
    assert oldbalance + dst_amount == newbalance


def test_cro_bridge_contract(cronos, chainmain, hermes):
    """
    test sending basetcro from cronos to crypto-org-chain using CroBridge contract.
    depends on `test_ibc` to send the original coins.
    """
    coin_receiver = chainmain.cosmos_cli().address("signer2")
    dst_amount = 2
    src_amount = dst_amount * (10**10)  # the decimal places difference
    oldbalance = get_balance(chainmain, coin_receiver, "basecro")

    # case 2: use CroBridge contract
    w3 = cronos.w3
    contract = deploy_contract(w3, CONTRACTS["CroBridge"])
    tx = contract.functions.send_cro_to_crypto_org(coin_receiver).buildTransaction(
        {"from": ADDRS["signer2"], "value": src_amount}
    )
    receipt = send_transaction(w3, tx)
    assert receipt.status == 1

    newbalance = 0

    def check_balance_change():
        nonlocal newbalance
        newbalance = get_balance(chainmain, coin_receiver, "basecro")
        return oldbalance != newbalance

    wait_for_fn("check balance change", check_balance_change)
    assert oldbalance + dst_amount == newbalance
