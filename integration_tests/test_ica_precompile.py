import re

import pytest

from .ibc_utils import (
    funds_ica,
    generate_ica_packet,
    prepare_network,
    wait_for_check_channel_ready,
    wait_for_check_tx,
)
from .utils import ADDRS, CONTRACTS, KEYS, deploy_contract, send_transaction


@pytest.fixture(scope="module")
def ibc(request, tmp_path_factory):
    "prepare-network"
    name = "ibc"
    path = tmp_path_factory.mktemp(name)
    network = prepare_network(path, name, connection_only=True)
    yield from network


def test_call(ibc):
    connid = "connection-0"
    cli_host = ibc.chainmain.cosmos_cli()
    cli_controller = ibc.cronos.cosmos_cli()

    w3 = ibc.cronos.w3
    addr = ADDRS["signer2"]
    keys = KEYS["signer2"]
    contract = deploy_contract(w3, CONTRACTS["TestICA"], (), keys)
    data = {"from": addr, "gas": 200000}

    print("register ica account")
    tx = contract.functions.nativeRegister(connid).build_transaction(data)
    receipt = send_transaction(w3, tx, keys)
    assert receipt.status == 1

    # TODO: parse from emitted event
    port_id = "icacontroller-crc1q04jewhxw4xxu3vlg3rc85240h9q7ns6hglz0g"
    channel_id = "channel-1"
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
    wait_for_check_tx(cli_host, ica_address, num_txs)
    # check if the funds are reduced in interchain account
    assert cli_host.balance(ica_address, denom="basecro") == 50000000
