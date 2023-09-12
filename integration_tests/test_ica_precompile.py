import json
import re

import pytest
from web3._utils.contracts import find_matching_event_abi
from web3._utils.events import get_event_data

from .ibc_utils import (
    funds_ica,
    generate_ica_packet,
    prepare_network,
    wait_for_check_channel_ready,
    wait_for_check_tx,
)
from .utils import (
    ADDRS,
    CONTRACTS,
    KEYS,
    deploy_contract,
    get_method_map,
    send_transaction,
)


@pytest.fixture(scope="module")
def ibc(request, tmp_path_factory):
    "prepare-network"
    name = "ibc"
    path = tmp_path_factory.mktemp(name)
    network = prepare_network(path, name, incentivized=False, connection_only=True)
    yield from network


def get_log_data(w3, method_map, info, logs):
    for _, log in enumerate(logs):
        method = method_map[log.topics[0].hex()]
        name = method.split("(")[0]
        event_abi = find_matching_event_abi(info, name)
        event_data = get_event_data(w3.codec, event_abi, log)
        return event_data["args"]["data"].decode("utf-8")


def test_call(ibc):
    connid = "connection-0"
    cli_host = ibc.chainmain.cosmos_cli()
    cli_controller = ibc.cronos.cosmos_cli()

    w3 = ibc.cronos.w3
    addr = ADDRS["signer2"]
    keys = KEYS["signer2"]
    jsonfile = CONTRACTS["TestICA"]
    contract = deploy_contract(w3, jsonfile, (), keys)
    data = {"from": addr, "gas": 200000}

    print("register ica account")
    tx = contract.functions.nativeRegister(connid).build_transaction(data)
    receipt = send_transaction(w3, tx, keys)
    assert receipt.status == 1
    info = json.loads(jsonfile.read_text())["abi"]
    method_map = get_method_map(info)
    res = get_log_data(w3, method_map, info, receipt.logs)
    res = re.sub(r"\n\t", "", res)
    channel_id, port_id = res.split("\x128")
    print("port-id", port_id, "channel-id", channel_id)

    wait_for_check_channel_ready(cli_controller, connid, channel_id)

    print("query ica account")
    res = contract.caller.nativeQueryAccount(connid, addr)
    ica_address = re.sub(r"\n>", "", res.decode("utf-8"))
    print("ica address", ica_address)

    funds_ica(cli_host, ica_address)
    num_txs = len(cli_host.query_all_txs(ica_address)["txs"])
    str = generate_ica_packet(cli_controller, ica_address, cli_host.address("signer2"))
    # submit transaction on host chain on behalf of interchain account
    tx = contract.functions.nativeSubmitMsgs(connid, str).build_transaction(data)
    receipt = send_transaction(w3, tx, keys)
    assert receipt.status == 1
    res = get_log_data(w3, method_map, info, receipt.logs)
    res = re.sub(r"\x08", "", res)
    print("seq", res)
    wait_for_check_tx(cli_host, ica_address, num_txs)
    # check if the funds are reduced in interchain account
    assert cli_host.balance(ica_address, denom="basecro") == 50000000
