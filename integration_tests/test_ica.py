import json

import pytest
from pystarport import cluster

from .ibc_utils import (
    Status,
    deploy_contract,
    funds_ica,
    gen_send_msg,
    parse_events_rpc,
    prepare_network,
    register_acc,
    wait_for_check_channel_ready,
    wait_for_check_tx,
    wait_for_status_change,
)
from .utils import CONTRACTS, wait_for_fn

pytestmark = pytest.mark.ica


@pytest.fixture(scope="module")
def ibc(request, tmp_path_factory):
    "prepare-network"
    name = "ibc_rly"
    path = tmp_path_factory.mktemp(name)
    yield from prepare_network(
        path,
        name,
        incentivized=False,
        connection_only=True,
        relayer=cluster.Relayer.RLY.value,
    )


def test_ica(ibc, tmp_path):
    connid = "connection-0"
    cli_host = ibc.chainmain.cosmos_cli()
    cli_controller = ibc.cronos.cosmos_cli()
    ica_address, channel_id = register_acc(cli_controller, connid)
    balance = funds_ica(cli_host, ica_address)
    to = cli_host.address("signer2")
    amount = 1000
    denom = "basecro"
    jsonfile = CONTRACTS["TestICA"]
    tcontract = deploy_contract(ibc.cronos.w3, jsonfile)
    memo = {"src_callback": {"address": tcontract.address}}
    timeout_in_ns = 6000000000
    seq = 1

    def generated_tx_packet(msg_num):
        # generate a transaction to send to host chain
        m = gen_send_msg(ica_address, to, denom, amount)
        msgs = []
        for i in range(msg_num):
            msgs.append(m)
        data = json.dumps(msgs)
        packet = cli_controller.ica_generate_packet_data(data, json.dumps(memo))
        return packet

    def send_tx(msg_num, gas="200000"):
        num_txs = len(cli_host.query_all_txs(ica_address)["txs"])
        generated_tx = json.dumps(generated_tx_packet(msg_num))
        # submit transaction on host chain on behalf of interchain account
        rsp = cli_controller.ica_ctrl_send_tx(
            connid,
            generated_tx,
            timeout_in_ns,
            gas=gas,
            from_="signer2",
        )
        assert rsp["code"] == 0, rsp["raw_log"]
        events = parse_events_rpc(rsp["events"])
        assert int(events.get("send_packet")["packet_sequence"]) == seq
        wait_for_check_tx(cli_host, ica_address, num_txs)

    msg_num = 10
    assert tcontract.caller.getStatus(channel_id, seq) == Status.PENDING
    send_tx(msg_num)
    balance -= amount * msg_num
    assert cli_host.balance(ica_address, denom=denom) == balance
    wait_for_status_change(tcontract, channel_id, seq, timeout_in_ns / 1e9)
    assert tcontract.caller.getStatus(channel_id, seq) == Status.PENDING

    def check_for_ack():
        criteria = "message.action=/ibc.core.channel.v1.MsgAcknowledgement"
        return cli_controller.tx_search(criteria)["txs"]

    txs = wait_for_fn("ack change", check_for_ack)
    events = parse_events_rpc(txs[0]["events"])
    err = events.get("ibc_src_callback")["callback_error"]
    assert "sender is not authenticated" in err, err

    def generated_tx_txt(msg_num):
        # generate a transaction to send to host chain
        generated_tx = tmp_path / "generated_tx.txt"
        m = gen_send_msg(ica_address, to, denom, amount)
        msgs = []
        for i in range(msg_num):
            msgs.append(m)
        generated_tx_msg = {
            "body": {
                "messages": msgs,
            },
        }
        generated_tx.write_text(json.dumps(generated_tx_msg))
        return generated_tx

    no_timeout = 60

    def submit_msgs(msg_num, timeout_in_s=no_timeout, gas="200000"):
        num_txs = len(cli_host.query_all_txs(ica_address)["txs"])
        # submit transaction on host chain on behalf of interchain account
        rsp = cli_controller.icaauth_submit_tx(
            connid,
            generated_tx_txt(msg_num),
            timeout_duration=f"{timeout_in_s}s",
            gas=gas,
            from_="signer2",
        )
        assert rsp["code"] == 0, rsp["raw_log"]
        timeout = timeout_in_s + 3 if timeout_in_s < no_timeout else None
        wait_for_check_tx(cli_host, ica_address, num_txs, timeout)

    # submit large txs to trigger timeout
    msg_num = 140
    submit_msgs(msg_num, 5, "600000")
    assert cli_host.balance(ica_address, denom=denom) == balance
    wait_for_check_channel_ready(cli_controller, connid, channel_id, "STATE_CLOSED")
    # reopen ica account after channel get closed
    ica_address2, channel_id2 = register_acc(cli_controller, connid)
    assert ica_address2 == ica_address, ica_address2
    assert channel_id2 != channel_id, channel_id2
    # submit normal txs should work
    msg_num = 2
    submit_msgs(msg_num)
    balance -= amount * msg_num
    # check if the funds are reduced in interchain account
    assert cli_host.balance(ica_address, denom=denom) == balance
