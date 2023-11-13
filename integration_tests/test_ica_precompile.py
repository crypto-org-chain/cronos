import base64
import json
from enum import IntEnum

import pytest
from pystarport import cluster
from web3.datastructures import AttributeDict

from .ibc_utils import (
    funds_ica,
    gen_send_msg,
    get_next_channel,
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
    send_transaction,
    wait_for_fn,
)

CONTRACT = "0x0000000000000000000000000000000000000066"
connid = "connection-0"
no_timeout = 300000000000
denom = "basecro"
keys = KEYS["signer2"]
validator = "validator"
amt = 1000


class Status(IntEnum):
    NONE, SUCCESS, FAIL = range(3)


@pytest.fixture(scope="module")
def ibc(request, tmp_path_factory):
    "prepare-network"
    name = "ibc_rly_evm"
    path = tmp_path_factory.mktemp(name)
    yield from prepare_network(
        path,
        name,
        incentivized=False,
        connection_only=True,
        relayer=cluster.Relayer.RLY.value,
    )


def register_acc(cli, w3, register, query, data, addr, channel_id):
    print(f"register ica account with {channel_id}")
    tx = register(connid, "").build_transaction(data)
    receipt = send_transaction(w3, tx, keys)
    assert receipt.status == 1
    owner = eth_to_bech32(addr)
    wait_for_check_channel_ready(cli, connid, channel_id)
    res = cli.ica_query_account(connid, owner)
    ica_address = res["interchain_account_address"]
    print("query ica account", ica_address)
    res = query(connid, addr).call()
    assert ica_address == res, res
    return ica_address


def submit_msgs(
    ibc,
    func,
    data,
    ica_address,
    add_delegate,
    expected_seq,
    event,
    timeout=no_timeout,
    amount=amt,
    need_wait=True,
    msg_num=2,
):
    cli_host = ibc.chainmain.cosmos_cli()
    cli_controller = ibc.cronos.cosmos_cli()
    w3 = ibc.cronos.w3
    to = cli_host.address(validator)
    # generate msgs send to host chain
    m = gen_send_msg(ica_address, to, denom, amount)
    msgs = []
    diff_amt = 0
    for i in range(msg_num):
        msgs.append(m)
        diff_amt += amount
    if add_delegate:
        to = cli_host.address(validator, bech="val")
        # generate msg delegate to host chain
        amt1 = 100
        msgs.append(
            {
                "@type": "/cosmos.staking.v1beta1.MsgDelegate",
                "delegator_address": ica_address,
                "validator_address": to,
                "amount": {"denom": denom, "amount": f"{amt1}"},
            }
        )
        diff_amt += amt1
    generated_packet = cli_controller.ica_generate_packet_data(json.dumps(msgs))
    num_txs = len(cli_host.query_all_txs(ica_address)["txs"])
    str = base64.b64decode(generated_packet["data"])
    # submit transaction on host chain on behalf of interchain account
    tx = func(connid, str, timeout).build_transaction(data)
    receipt = send_transaction(w3, tx, keys)
    assert receipt.status == 1
    if timeout < no_timeout:
        timeout_in_s = timeout / 1e9 + 1
        print(f"wait for {timeout_in_s}s")
        wait_for_check_tx(cli_host, ica_address, num_txs, timeout_in_s)
    else:
        logs = event.getLogs()
        assert len(logs) > 0
        assert logs[0].args == AttributeDict({"seq": expected_seq})
        if need_wait:
            wait_for_check_tx(cli_host, ica_address, num_txs)
    return str, diff_amt


def test_call(ibc):
    cli_host = ibc.chainmain.cosmos_cli()
    cli_controller = ibc.cronos.cosmos_cli()
    w3 = ibc.cronos.w3
    name = "signer2"
    addr = ADDRS[name]
    contract_info = json.loads(CONTRACT_ABIS["IICAModule"].read_text())
    contract = w3.eth.contract(address=CONTRACT, abi=contract_info)
    data = {"from": ADDRS[name]}
    ica_address = register_acc(
        cli_controller,
        w3,
        contract.functions.registerAccount,
        contract.functions.queryAccount,
        data,
        addr,
        get_next_channel(cli_controller, connid),
    )
    balance = funds_ica(cli_host, ica_address)
    expected_seq = 1
    _, diff = submit_msgs(
        ibc,
        contract.functions.submitMsgs,
        data,
        ica_address,
        False,
        expected_seq,
        contract.events.SubmitMsgsResult,
    )
    balance -= diff
    assert cli_host.balance(ica_address, denom=denom) == balance
    expected_seq += 1
    _, diff = submit_msgs(
        ibc,
        contract.functions.submitMsgs,
        data,
        ica_address,
        True,
        expected_seq,
        contract.events.SubmitMsgsResult,
    )
    balance -= diff
    assert cli_host.balance(ica_address, denom=denom) == balance


def wait_for_status_change(tcontract, seq):
    print(f"wait for status change for {seq}")

    def check_status():
        status = tcontract.caller.statusMap(seq)
        print("current", status)
        return status

    wait_for_fn("current status", check_status)


def wait_for_packet_log(event, seq, status):
    print("wait for log arrive", seq, status)
    expected = AttributeDict({"seq": seq, "status": status})

    def check_log():
        logs = event.getLogs()
        return len(logs) > 0 and logs[0].args == expected

    wait_for_fn("packet log", check_log)


def test_sc_call(ibc):
    cli_host = ibc.chainmain.cosmos_cli()
    cli_controller = ibc.cronos.cosmos_cli()
    w3 = ibc.cronos.w3
    contract_info = json.loads(CONTRACT_ABIS["IICAModule"].read_text())
    contract = w3.eth.contract(address=CONTRACT, abi=contract_info)
    jsonfile = CONTRACTS["TestICA"]
    tcontract = deploy_contract(w3, jsonfile)
    addr = tcontract.address
    name = "signer2"
    signer = ADDRS[name]
    keys = KEYS[name]
    default_gas = 400000
    data = {"from": signer, "gas": default_gas}
    channel_id = get_next_channel(cli_controller, connid)
    ica_address = register_acc(
        cli_controller,
        w3,
        tcontract.functions.callRegister,
        contract.functions.queryAccount,
        data,
        addr,
        channel_id,
    )
    balance = funds_ica(cli_host, ica_address)
    assert tcontract.caller.getAccount() == signer
    assert tcontract.functions.callQueryAccount(connid, addr).call() == ica_address

    # register from another user should fail
    name = "signer1"
    data = {"from": ADDRS[name], "gas": default_gas}
    version = ""
    tx = tcontract.functions.callRegister(connid, version).build_transaction(data)
    res = send_transaction(w3, tx, KEYS[name])
    assert res.status == 0
    assert tcontract.caller.getAccount() == signer

    assert tcontract.functions.delegateQueryAccount(connid, addr).call() == ica_address
    assert tcontract.functions.staticQueryAccount(connid, addr).call() == ica_address

    # readonly call should fail
    def register_ro(func):
        tx = func(connid, version).build_transaction(data)
        assert send_transaction(w3, tx, keys).status == 0

    register_ro(tcontract.functions.delegateRegister)
    register_ro(tcontract.functions.staticRegister)

    # readonly call should fail
    def submit_msgs_ro(func, str):
        tx = func(connid, str, no_timeout).build_transaction(data)
        assert send_transaction(w3, tx, keys).status == 0

    expected_seq = 1
    str, diff = submit_msgs(
        ibc,
        tcontract.functions.callSubmitMsgs,
        data,
        ica_address,
        False,
        expected_seq,
        contract.events.SubmitMsgsResult,
    )
    submit_msgs_ro(tcontract.functions.delegateSubmitMsgs, str)
    submit_msgs_ro(tcontract.functions.staticSubmitMsgs, str)
    last_seq = tcontract.caller.getLastSeq()
    wait_for_status_change(tcontract, last_seq)
    status = tcontract.caller.statusMap(last_seq)
    assert expected_seq == last_seq
    assert status == Status.SUCCESS
    wait_for_packet_log(tcontract.events.OnPacketResult, last_seq, status)
    balance -= diff
    assert cli_host.balance(ica_address, denom=denom) == balance

    expected_seq += 1
    str, diff = submit_msgs(
        ibc,
        tcontract.functions.callSubmitMsgs,
        data,
        ica_address,
        True,
        expected_seq,
        contract.events.SubmitMsgsResult,
    )
    submit_msgs_ro(tcontract.functions.delegateSubmitMsgs, str)
    submit_msgs_ro(tcontract.functions.staticSubmitMsgs, str)
    last_seq = tcontract.caller.getLastSeq()
    wait_for_status_change(tcontract, last_seq)
    status = tcontract.caller.statusMap(last_seq)
    assert expected_seq == last_seq
    assert status == Status.SUCCESS
    wait_for_packet_log(tcontract.events.OnPacketResult, last_seq, status)
    balance -= diff
    assert cli_host.balance(ica_address, denom=denom) == balance

    expected_seq += 1
    # balance should not change on fail
    submit_msgs(
        ibc,
        tcontract.functions.callSubmitMsgs,
        data,
        ica_address,
        False,
        expected_seq,
        contract.events.SubmitMsgsResult,
        amount=100000001,
        need_wait=False,
    )
    last_seq = tcontract.caller.getLastSeq()
    wait_for_status_change(tcontract, last_seq)
    status = tcontract.caller.statusMap(last_seq)
    assert expected_seq == last_seq
    assert status == Status.FAIL
    wait_for_packet_log(tcontract.events.OnPacketResult, last_seq, status)
    assert cli_host.balance(ica_address, denom=denom) == balance

    # balance should not change on timeout
    expected_seq += 1
    timeout = 5000000000
    data["gas"] = 800000
    submit_msgs(
        ibc,
        tcontract.functions.callSubmitMsgs,
        data,
        ica_address,
        False,
        expected_seq,
        contract.events.SubmitMsgsResult,
        timeout,
        msg_num=100,
    )
    last_seq = tcontract.caller.getLastSeq()
    wait_for_status_change(tcontract, last_seq)
    status = tcontract.caller.statusMap(last_seq)
    assert expected_seq == last_seq
    assert status == Status.FAIL
    wait_for_packet_log(tcontract.events.OnPacketResult, last_seq, status)
    assert cli_host.balance(ica_address, denom=denom) == balance
    wait_for_check_channel_ready(cli_controller, connid, channel_id, "STATE_CLOSED")
    data["gas"] = default_gas
    ica_address2 = register_acc(
        cli_controller,
        w3,
        tcontract.functions.callRegister,
        contract.functions.queryAccount,
        data,
        addr,
        get_next_channel(cli_controller, connid),
    )
    assert ica_address2 == ica_address, ica_address2
    expected_seq = 1
    str, diff = submit_msgs(
        ibc,
        tcontract.functions.callSubmitMsgs,
        data,
        ica_address,
        False,
        expected_seq,
        contract.events.SubmitMsgsResult,
    )
    last_seq = tcontract.caller.getLastSeq()
    wait_for_status_change(tcontract, last_seq)
    status = tcontract.caller.statusMap(last_seq)
    assert expected_seq == last_seq
    assert status == Status.SUCCESS
    # wait for ack to add log from call evm
    wait_for_packet_log(tcontract.events.OnPacketResult, last_seq, status)
    balance -= diff
    assert cli_host.balance(ica_address, denom=denom) == balance
