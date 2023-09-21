import json
import re

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
    CONTRACTS,
    KEYS,
    deploy_contract,
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


def generate_msg_send(ica_address, to, denom, amt):
    return {
        "@type": "/cosmos.bank.v1beta1.MsgSend",
        "from_address": ica_address,
        "to_address": to,
        "amount": [{"denom": denom, "amount": f"{amt}"}],
    }


def generate_msg_delegate(ica_address, to, denom, amt):
    return {
        "@type": "/cosmos.staking.v1beta1.MsgDelegate",
        "delegator_address": ica_address,
        "validator_address": to,
        "amount": {"denom": denom, "amount": f"{amt}"},
    }


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

    print("register ica account from", contract.address)
    tx = contract.functions.nativeRegister(connid).build_transaction(data)
    receipt = send_transaction(w3, tx, keys)
    assert receipt.status == 1
    owner = eth_to_bech32(addr)
    channel_id = "channel-0"
    wait_for_check_channel_ready(cli_controller, connid, channel_id)
    res = cli_controller.ica_query_account(connid, owner)
    ica_address = res["interchain_account_address"]
    print("query ica account", ica_address)
    res = contract.caller.nativeQueryAccount(connid, addr)
    res = re.sub(r"\n>", "", res.decode("utf-8"))
    assert ica_address == res, res
    balance = funds_ica(cli_host, ica_address)

    name = "validator"
    denom = "basecro"
    amt = 1000

    def submit_msgs(msgs, seq):
        generated_packet = cli_controller.ica_generate_packet_data(msgs)
        num_txs = len(cli_host.query_all_txs(ica_address)["txs"])
        start = w3.eth.get_block_number()
        str = json.dumps(generated_packet)
        # submit transaction on host chain on behalf of interchain account
        tx = contract.functions.nativeSubmitMsgs(connid, str).build_transaction(data)
        receipt = send_transaction(w3, tx, keys)
        assert receipt.status == 1
        logs = get_logs_since(w3, CONTRACT, start)
        expected = [{"seq": seq}]
        for i, log in enumerate(logs):
            method_name, args = get_topic_data(w3, method_map, contract_info, log)
            assert args == AttributeDict(expected[i]), [i, method_name]
        wait_for_check_tx(cli_host, ica_address, num_txs)

    # generate msg send tx to host chain
    msg_send = generate_msg_send(
        ica_address,
        cli_host.address(name),
        denom,
        amt,
    )
    submit_msgs(json.dumps(msg_send), "1")
    balance -= amt
    assert cli_host.balance(ica_address, denom=denom) == balance
    # generate multi msgs to host chain
    amt1 = 100
    msg_delegate = generate_msg_delegate(
        ica_address,
        cli_host.address(name, bech="val"),
        denom,
        amt1,
    )
    submit_msgs(json.dumps([msg_send, msg_delegate]), "2")
    balance -= amt
    balance -= amt1
    assert cli_host.balance(ica_address, denom=denom) == balance
