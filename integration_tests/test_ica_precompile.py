import json

import pytest
from web3.datastructures import AttributeDict

from .ibc_utils import (
    funds_ica,
    prepare_network,
    wait_for_check_channel_ready,
    wait_for_check_tx,
)
from .utils import (
    ADDRS,
    CONTRACT_ABIS,
    KEYS,
    eth_to_bech32,
    get_logs_since,
    get_method_map,
    get_topic_data,
    send_transaction,
)

CONTRACT = "0x0000000000000000000000000000000000000066"
contract_info = json.loads(CONTRACT_ABIS["IICAModule"].read_text())
method_map = get_method_map(contract_info)


@pytest.fixture(scope="module")
def ibc(request, tmp_path_factory):
    "prepare-network"
    name = "ibc"
    path = tmp_path_factory.mktemp(name)
    network = prepare_network(path, name, incentivized=False, connection_only=True)
    yield from network


def test_call(ibc, tmp_path):
    connid = "connection-0"
    cli_host = ibc.chainmain.cosmos_cli()
    cli_controller = ibc.cronos.cosmos_cli()
    gas = 400000
    fees = "100000000basetcro"
    rsp = cli_controller.icaauth_register_account(
        connid, from_="signer2", gas=gas, fees=fees, print_proto_only=True
    ).rstrip(b"\n")
    w3 = ibc.cronos.w3
    addr = ADDRS["signer2"]
    keys = KEYS["signer2"]
    tx = {"from": addr, "to": CONTRACT, "gas": gas, "data": rsp}
    print("register ica account")
    start = w3.eth.get_block_number()
    receipt = send_transaction(w3, tx, keys)
    assert receipt.status == 1
    logs = get_logs_since(w3, CONTRACT, start)
    owner = eth_to_bech32(addr)
    channel_id = "channel-0"
    port_id = f"icacontroller-{owner}"
    expected = [{"channelId": channel_id, "portId": port_id}]
    for i, log in enumerate(logs):
        method_name, args = get_topic_data(w3, method_map, contract_info, log)
        assert args == AttributeDict(expected[i]), [i, method_name]

    wait_for_check_channel_ready(cli_controller, connid, channel_id)
    res = cli_controller.ica_query_account(connid, owner)
    ica_address = res["interchain_account_address"]
    print("ica account", ica_address)

    funds_ica(cli_host, ica_address)
    num_txs = len(cli_host.query_all_txs(ica_address)["txs"])
    # generate a transaction to send to host chain
    to = cli_host.address("signer2")
    generated_tx_msg = {
        "@type": "/cosmos.bank.v1beta1.MsgSend",
        "from_address": ica_address,
        "to_address": to,
        "amount": [{"denom": "basecro", "amount": "50000000"}],
    }
    str = json.dumps(generated_tx_msg)
    generated_packet = cli_controller.ica_generate_packet_data(str)
    packet_data_str = json.dumps(generated_packet)
    start = w3.eth.get_block_number()
    # submit transaction on host chain on behalf of interchain account
    rsp = cli_controller.icaauth_print_submit_tx_proto(
        connid,
        from_="signer2",
        packet_data_str=packet_data_str,
    ).rstrip(b"\n")
    tx = {"from": addr, "to": CONTRACT, "gas": gas, "data": rsp}
    receipt = send_transaction(w3, tx, keys)
    assert receipt.status == 1
    logs = get_logs_since(w3, CONTRACT, start)
    expected = [{"seq": "1"}]
    for i, log in enumerate(logs):
        method_name, args = get_topic_data(w3, method_map, contract_info, log)
        assert args == AttributeDict(expected[i]), [i, method_name]

    wait_for_check_tx(cli_host, ica_address, num_txs)
    # check if the funds are reduced in interchain account
    assert cli_host.balance(ica_address, denom="basecro") == 50000000
