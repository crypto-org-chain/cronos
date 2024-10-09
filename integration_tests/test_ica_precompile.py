import base64
import json
from enum import Enum

import pytest
from eth_utils import keccak
from pystarport import cluster
from web3.datastructures import AttributeDict

from .ibc_utils import (
    Status,
    funds_ica,
    gen_send_msg,
    get_next_channel,
    prepare_network,
    wait_for_check_channel_ready,
    wait_for_check_tx,
    wait_for_status_change,
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

pytestmark = pytest.mark.ica

CONTRACT = "0x0000000000000000000000000000000000000066"
connid = "connection-0"
no_timeout = 300000000000
denom = "basecro"
validator = "validator"
amt = 1000


class Ordering(Enum):
    NONE = 0
    UNORDERED = 1
    ORDERED = 2


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


def register_acc(
    cli,
    w3,
    register,
    query,
    data,
    channel_id,
    order,
    signer="signer2",
    expect_status=1,
    contract_addr=None,
):
    addr = contract_addr if contract_addr else ADDRS[signer]
    keys = KEYS[signer]
    print(f"register ica account with {channel_id}")
    tx = register(connid, "", order).build_transaction(data)
    receipt = send_transaction(w3, tx, keys)
    assert receipt.status == expect_status
    owner = eth_to_bech32(addr)
    if expect_status == 1:
        wait_for_check_channel_ready(cli, connid, channel_id)
    res = cli.ica_query_account(connid, owner)
    ica_address = res["address"]
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
    channel_id,
    timeout=no_timeout,
    amount=amt,
    need_wait=True,
    msg_num=2,
    with_channel_id=True,
    signer="signer2",
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
    if with_channel_id:
        tx = func(connid, channel_id, str, timeout).build_transaction(data)
    else:
        tx = func(connid, str, timeout).build_transaction(data)
    receipt = send_transaction(w3, tx, KEYS[signer])
    assert receipt.status == 1
    if timeout < no_timeout:
        timeout_in_s = timeout / 1e9 + 1
        print(f"wait for {timeout_in_s}s")
        wait_for_check_tx(cli_host, ica_address, num_txs, timeout_in_s)
    else:
        logs = event.get_logs()
        assert len(logs) > 0
        assert logs[0].args == AttributeDict(
            {
                "packetSrcChannel": keccak(text=channel_id),
                "seq": expected_seq,
            }
        )
        if need_wait:
            wait_for_check_tx(cli_host, ica_address, num_txs)
    return str, diff_amt


@pytest.mark.parametrize("order", [Ordering.ORDERED.value, Ordering.UNORDERED.value])
def test_call(ibc, order):
    signer = "signer2" if order == Ordering.ORDERED.value else "community"
    cli_host = ibc.chainmain.cosmos_cli()
    cli_controller = ibc.cronos.cosmos_cli()
    w3 = ibc.cronos.w3
    contract_info = json.loads(CONTRACT_ABIS["IICAModule"].read_text())
    contract = w3.eth.contract(address=CONTRACT, abi=contract_info)
    data = {"from": ADDRS[signer]}
    channel_id = get_next_channel(cli_controller, connid)
    ica_address = register_acc(
        cli_controller,
        w3,
        contract.functions.registerAccount,
        contract.functions.queryAccount,
        data,
        channel_id,
        order,
        signer=signer,
    )
    balance = funds_ica(cli_host, ica_address, signer=signer)
    expected_seq = 1
    _, diff = submit_msgs(
        ibc,
        contract.functions.submitMsgs,
        data,
        ica_address,
        False,
        expected_seq,
        contract.events.SubmitMsgsResult,
        channel_id,
        with_channel_id=False,
        signer=signer,
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
        channel_id,
        with_channel_id=False,
        signer=signer,
    )
    balance -= diff
    assert cli_host.balance(ica_address, denom=denom) == balance


def wait_for_packet_log(start, event, channel_id, seq, status):
    print("wait for log arrive", seq, status)
    expected = AttributeDict(
        {
            "packetSrcChannel": keccak(text=channel_id),
            "seq": seq,
            "status": status,
        }
    )

    def check_log():
        logs = event.get_logs(fromBlock=start)
        return len(logs) > 0 and logs[-1].args == expected

    wait_for_fn("packet log", check_log)


@pytest.mark.parametrize("order", [Ordering.ORDERED.value, Ordering.UNORDERED.value])
def test_sc_call(ibc, order):
    cli_host = ibc.chainmain.cosmos_cli()
    cli_controller = ibc.cronos.cosmos_cli()
    w3 = ibc.cronos.w3
    contract_info = json.loads(CONTRACT_ABIS["IICAModule"].read_text())
    contract = w3.eth.contract(address=CONTRACT, abi=contract_info)
    jsonfile = CONTRACTS["TestICA"]
    tcontract = deploy_contract(w3, jsonfile)
    contract_addr = tcontract.address
    signer = "signer2" if order == Ordering.ORDERED.value else "community"
    addr = ADDRS[signer]
    keys = KEYS[signer]
    default_gas = 500000
    data = {"from": addr, "gas": default_gas}
    channel_id = get_next_channel(cli_controller, connid)
    ica_address = register_acc(
        cli_controller,
        w3,
        tcontract.functions.callRegister,
        contract.functions.queryAccount,
        data,
        channel_id,
        order,
        signer=signer,
        contract_addr=contract_addr,
    )
    assert tcontract.caller.getAccount() == addr
    assert (
        tcontract.functions.callQueryAccount(connid, contract_addr).call()
        == ica_address
    )

    # register from another user should fail
    signer1 = "signer1"
    data = {"from": ADDRS[signer1], "gas": default_gas}
    version = ""
    tx = tcontract.functions.callRegister(
        connid,
        version,
        order,
    ).build_transaction(data)
    res = send_transaction(w3, tx, KEYS[signer1])
    assert res.status == 0
    assert tcontract.caller.getAccount() == addr

    assert (
        tcontract.functions.delegateQueryAccount(connid, contract_addr).call()
        == ica_address
    )
    assert (
        tcontract.functions.staticQueryAccount(connid, contract_addr).call()
        == ica_address
    )

    # readonly call should fail
    def register_ro(func):
        tx = func(connid, version, order).build_transaction(data)
        assert send_transaction(w3, tx, keys).status == 0

    register_ro(tcontract.functions.delegateRegister)
    register_ro(tcontract.functions.staticRegister)

    # readonly call should fail
    def submit_msgs_ro(func, str):
        tx = func(connid, str, no_timeout).build_transaction(data)
        assert send_transaction(w3, tx, keys).status == 0

    expected_seq = 1
    packet_event = tcontract.events.OnPacketResult
    start = w3.eth.get_block_number()
    str, diff = submit_msgs(
        ibc,
        tcontract.functions.callSubmitMsgs,
        data,
        ica_address,
        False,
        expected_seq,
        contract.events.SubmitMsgsResult,
        channel_id,
        need_wait=False,
        signer=signer,
    )
    submit_msgs_ro(tcontract.functions.delegateSubmitMsgs, str)
    submit_msgs_ro(tcontract.functions.staticSubmitMsgs, str)
    last_seq = tcontract.caller.getLastSeq()
    wait_for_status_change(tcontract, channel_id, last_seq)
    status = tcontract.caller.getStatus(channel_id, last_seq)
    assert expected_seq == last_seq
    assert status == Status.FAIL
    wait_for_packet_log(start, packet_event, channel_id, last_seq, status)

    expected_seq += 1
    balance = funds_ica(cli_host, ica_address, signer=signer)
    start = w3.eth.get_block_number()
    str, diff = submit_msgs(
        ibc,
        tcontract.functions.callSubmitMsgs,
        data,
        ica_address,
        False,
        expected_seq,
        contract.events.SubmitMsgsResult,
        channel_id,
        signer=signer,
    )
    last_seq = tcontract.caller.getLastSeq()
    wait_for_status_change(tcontract, channel_id, last_seq)
    status = tcontract.caller.getStatus(channel_id, last_seq)
    assert expected_seq == last_seq
    assert status == Status.SUCCESS
    wait_for_packet_log(start, packet_event, channel_id, last_seq, status)
    balance -= diff
    assert cli_host.balance(ica_address, denom=denom) == balance

    expected_seq += 1
    start = w3.eth.get_block_number()
    str, diff = submit_msgs(
        ibc,
        tcontract.functions.callSubmitMsgs,
        data,
        ica_address,
        True,
        expected_seq,
        contract.events.SubmitMsgsResult,
        channel_id,
        signer=signer,
    )
    submit_msgs_ro(tcontract.functions.delegateSubmitMsgs, str)
    submit_msgs_ro(tcontract.functions.staticSubmitMsgs, str)
    last_seq = tcontract.caller.getLastSeq()
    wait_for_status_change(tcontract, channel_id, last_seq)
    status = tcontract.caller.getStatus(channel_id, last_seq)
    assert expected_seq == last_seq
    assert status == Status.SUCCESS
    wait_for_packet_log(start, packet_event, channel_id, last_seq, status)
    balance -= diff
    assert cli_host.balance(ica_address, denom=denom) == balance

    expected_seq += 1
    start = w3.eth.get_block_number()
    # balance should not change on fail
    submit_msgs(
        ibc,
        tcontract.functions.callSubmitMsgs,
        data,
        ica_address,
        False,
        expected_seq,
        contract.events.SubmitMsgsResult,
        channel_id,
        amount=100000001,
        need_wait=False,
        signer=signer,
    )
    last_seq = tcontract.caller.getLastSeq()
    wait_for_status_change(tcontract, channel_id, last_seq)
    status = tcontract.caller.getStatus(channel_id, last_seq)
    assert expected_seq == last_seq
    assert status == Status.FAIL
    wait_for_packet_log(start, packet_event, channel_id, last_seq, status)
    assert cli_host.balance(ica_address, denom=denom) == balance

    # balance should not change on timeout
    expected_seq += 1
    start = w3.eth.get_block_number()
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
        channel_id,
        timeout,
        msg_num=100,
        signer=signer,
    )
    last_seq = tcontract.caller.getLastSeq()
    wait_for_status_change(tcontract, channel_id, last_seq)
    status = tcontract.caller.getStatus(channel_id, last_seq)
    assert expected_seq == last_seq
    assert status == Status.FAIL
    wait_for_packet_log(start, packet_event, channel_id, last_seq, status)
    assert cli_host.balance(ica_address, denom=denom) == balance

    if order == Ordering.UNORDERED.value:
        channel_id2 = get_next_channel(cli_controller, connid)
        register_acc(
            cli_controller,
            w3,
            tcontract.functions.callRegister,
            contract.functions.queryAccount,
            data,
            channel_id2,
            order,
            expect_status=0,
            signer=signer,
            contract_addr=contract_addr,
        )
        channel_id2 = channel_id
        expected_seq += 1
    else:
        wait_for_check_channel_ready(cli_controller, connid, channel_id, "STATE_CLOSED")
        data["gas"] = default_gas
        channel_id2 = get_next_channel(cli_controller, connid)
        ica_address2 = register_acc(
            cli_controller,
            w3,
            tcontract.functions.callRegister,
            contract.functions.queryAccount,
            data,
            channel_id2,
            order,
            signer=signer,
            contract_addr=contract_addr,
        )
        assert channel_id2 != channel_id, channel_id2
        assert ica_address2 == ica_address, ica_address2
        expected_seq = 1
    start = w3.eth.get_block_number()
    str, diff = submit_msgs(
        ibc,
        tcontract.functions.callSubmitMsgs,
        data,
        ica_address,
        False,
        expected_seq,
        contract.events.SubmitMsgsResult,
        channel_id2,
        signer=signer,
    )
    last_seq = tcontract.caller.getLastSeq()
    wait_for_status_change(tcontract, channel_id2, last_seq)
    status = tcontract.caller.getStatus(channel_id2, last_seq)
    assert expected_seq == last_seq
    assert status == Status.SUCCESS
    # wait for ack to add log from call evm
    wait_for_packet_log(start, packet_event, channel_id2, last_seq, status)
    balance -= diff
    assert cli_host.balance(ica_address, denom=denom) == balance
