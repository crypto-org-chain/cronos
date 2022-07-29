import base64
import json
import subprocess
from pathlib import Path
from typing import NamedTuple

import pytest
from pystarport import ports

from .network import Chainmain, Cronos, Hermes, setup_custom_cronos
from .utils import (
    ADDRS,
    CONTRACTS,
    deploy_contract,
    eth_to_bech32,
    send_transaction,
    supervisorctl,
    wait_for_fn,
    wait_for_new_blocks,
    wait_for_port,
)


class IBCNetwork(NamedTuple):
    cronos: Cronos
    chainmain: Chainmain
    hermes: Hermes


@pytest.fixture(scope="module")
def ibc(request, tmp_path_factory):
    "start-cronos"
    path = tmp_path_factory.mktemp("ibc")
    gen = setup_custom_cronos(
        path, 26700, Path(__file__).parent / "configs/ibc.jsonnet"
    )
    cronos = next(gen)
    try:
        chainmain = Chainmain(cronos.base_dir.parent / "chainmain-1")
        hermes = Hermes(cronos.base_dir.parent / "relayer.toml")
        # wait for grpc ready
        wait_for_port(ports.grpc_port(chainmain.base_port(0)))  # chainmain grpc
        wait_for_port(ports.grpc_port(cronos.base_port(0)))  # cronos grpc
        subprocess.check_call(
            [
                "hermes",
                "--config",
                hermes.configpath,
                "create",
                "channel",
                "--port-a",
                "transfer",
                "--port-b",
                "transfer",
                "cronos_777-1",
                "--chain-b",
                "chainmain-1",
                "--new-client-connection",
            ]
        )
        # cmd = (
        #     f"yes | hermes --config {hermes.configpath} create channel "
        #     f"--port-a transfer --port-b transfer cronos_777-1 --chain-b chainmain-1 "
        #     f"--new-client-connection"
        # )
        # subprocess.run(cmd, check=True, shell=True)
        supervisorctl(cronos.base_dir / "../tasks.ini", "start", "relayer-demo")
        wait_for_port(hermes.port)
        yield IBCNetwork(cronos, chainmain, hermes)
    finally:
        try:
            next(gen)
        except StopIteration:
            pass


def get_balance(chain, addr, denom):
    return chain.cosmos_cli().balance(addr, denom)


def test_ibc(ibc):
    "test sending basecro from crypto-org chain to cronos"
    # wait for hermes
    output = subprocess.getoutput(
        f"curl -s -X GET 'http://127.0.0.1:{ibc.hermes.port}/state' | jq"
    )
    assert json.loads(output)["status"] == "success"

    my_ibc0 = "chainmain-1"
    my_ibc1 = "cronos_777-1"
    my_channel = "channel-0"
    my_config = ibc.hermes.configpath
    # signer2
    coin_receiver = eth_to_bech32(ADDRS["signer2"])
    src_amount = 10
    dst_amount = src_amount * (10**10)  # the decimal places difference
    src_denom = "basecro"
    dst_denom = "basetcro"
    # dstchainid srcchainid srcportid srchannelid
    # chainmain-1 -> cronos_777-1
    cmd = (
        f"hermes --config {my_config} tx raw ft-transfer "
        f"{my_ibc1} {my_ibc0} transfer {my_channel} {src_amount} "
        f"-o 1000 -n 1 -d {src_denom} -r {coin_receiver} -k relayer"
    )
    subprocess.run(cmd, check=True, shell=True)
    dstaddr = f"{coin_receiver}"
    olddstbalance = get_balance(ibc.cronos, dstaddr, dst_denom)
    newdstbalance = 0

    def check_balance_change():
        nonlocal newdstbalance
        newdstbalance = get_balance(ibc.cronos, dstaddr, dst_denom)
        return newdstbalance != olddstbalance

    wait_for_fn("check balance change", check_balance_change)
    expectedbalance = olddstbalance + dst_amount
    assert expectedbalance == newdstbalance


def test_cronos_transfer_tokens(ibc):
    """
    test sending basetcro from cronos to crypto-org-chain using cli transfer_tokens.
    depends on `test_ibc` to send the original coins.
    """
    output = subprocess.getoutput(
        f"curl -s -X GET 'http://127.0.0.1:{ibc.hermes.port}/state' | jq"
    )
    assert json.loads(output)["status"] == "success"

    coin_receiver = ibc.chainmain.cosmos_cli().address("signer2")
    dst_amount = 2
    src_amount = dst_amount * (10**10)  # the decimal places difference

    # case 1: use cronos cli
    oldbalance = get_balance(ibc.chainmain, coin_receiver, "basecro")
    cli = ibc.cronos.cosmos_cli()
    rsp = cli.transfer_tokens(
        cli.address("signer2"),
        coin_receiver,
        f"{src_amount}basetcro",
    )
    assert rsp["code"] == 0, rsp["raw_log"]

    newbalance = 0

    def check_balance_change():
        nonlocal newbalance
        newbalance = get_balance(ibc.chainmain, coin_receiver, "basecro")
        return oldbalance != newbalance

    wait_for_fn("check balance change", check_balance_change)
    assert oldbalance + dst_amount == newbalance


def test_cro_bridge_contract(ibc):
    """
    test sending basetcro from cronos to crypto-org-chain using CroBridge contract.
    depends on `test_ibc` to send the original coins.
    """
    coin_receiver = ibc.chainmain.cosmos_cli().address("signer2")
    dst_amount = 2
    src_amount = dst_amount * (10**10)  # the decimal places difference
    oldbalance = get_balance(ibc.chainmain, coin_receiver, "basecro")

    # case 2: use CroBridge contract
    w3 = ibc.cronos.w3
    contract = deploy_contract(w3, CONTRACTS["CroBridge"])
    tx = contract.functions.send_cro_to_crypto_org(coin_receiver).buildTransaction(
        {"from": ADDRS["signer2"], "value": src_amount}
    )
    receipt = send_transaction(w3, tx)
    assert receipt.status == 1

    newbalance = 0

    def check_balance_change():
        nonlocal newbalance
        newbalance = get_balance(ibc.chainmain, coin_receiver, "basecro")
        return oldbalance != newbalance

    wait_for_fn("check balance change", check_balance_change)
    assert oldbalance + dst_amount == newbalance


def test_ica(ibc, tmp_path):
    connid = "connection-0"
    cli_host = ibc.chainmain.cosmos_cli()
    cli_controller = ibc.cronos.cosmos_cli()

    print("register ica account")
    rsp = cli_controller.ica_register_account(
        connid, from_="signer2", gas="400000", fees="100000000basetcro"
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    port_id, channel_id = next(
        (
            base64.b64decode(evt["attributes"][0]["value"].encode()).decode(),
            base64.b64decode(evt["attributes"][1]["value"].encode()).decode(),
        )
        for evt in rsp["events"]
        if evt["type"] == "channel_open_init"
    )
    print("port-id", port_id, "channel-id", channel_id)

    print("wait for ica channel ready")

    def check_channel_ready():
        channels = cli_controller.ibc_query_channels(connid)["channels"]
        try:
            state = next(
                channel["state"]
                for channel in channels
                if channel["channel_id"] == channel_id
            )
        except StopIteration:
            return False
        return state == "STATE_OPEN"

    wait_for_fn("channel ready", check_channel_ready)

    print("query ica account")
    ica_address = cli_controller.ica_query_account(
        connid, cli_controller.address("signer2")
    )["interchainAccountAddress"]
    print("ica address", ica_address)

    # initial balance of interchain account should be zero
    assert cli_host.balance(ica_address) == 0

    # send some funds to interchain account
    rsp = cli_host.transfer("signer2", ica_address, "1cro", gas_prices="1000000basecro")
    assert rsp["code"] == 0, rsp["raw_log"]
    wait_for_new_blocks(cli_host, 1)

    # check if the funds are received in interchain account
    assert cli_host.balance(ica_address, denom="basecro") == 100000000

    # generate a transaction to send to host chain
    generated_tx = tmp_path / "generated_tx.txt"
    generated_tx_msg = cli_host.transfer(
        ica_address, cli_host.address("signer2"), "0.5cro", generate_only=True
    )

    print(generated_tx_msg)
    generated_tx.write_text(json.dumps(generated_tx_msg))

    num_txs = len(cli_host.query_all_txs(ica_address)["txs"])

    # submit transaction on host chain on behalf of interchain account
    rsp = cli_controller.ica_submit_tx(
        connid,
        generated_tx,
        from_="signer2",
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    packet_seq = next(
        int(base64.b64decode(evt["attributes"][4]["value"].encode()))
        for evt in rsp["events"]
        if evt["type"] == "send_packet"
    )
    print("packet sequence", packet_seq)

    def check_ica_tx():
        return len(cli_host.query_all_txs(ica_address)["txs"]) > num_txs

    print("wait for ica tx arrive")
    wait_for_fn("ica transfer tx", check_ica_tx)

    # check if the funds are reduced in interchain account
    assert cli_host.balance(ica_address, denom="basecro") == 50000000
