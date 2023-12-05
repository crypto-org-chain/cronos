import json

import pytest
from pystarport import cluster

from .ibc_utils import (
    deploy_contract,
    funds_ica,
    gen_send_msg,
    parse_events_rpc,
    prepare_network,
    register_acc,
    wait_for_check_tx,
)
from .utils import CONTRACTS, wait_for_fn


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


def test_cb(ibc):
    connid = "connection-0"
    cli_host = ibc.chainmain.cosmos_cli()
    cli_controller = ibc.cronos.cosmos_cli()
    ica_address, _ = register_acc(cli_controller, connid)
    funds_ica(cli_host, ica_address)
    num_txs = len(cli_host.query_all_txs(ica_address)["txs"])
    to = cli_host.address("signer2")
    amount = 1000
    denom = "basecro"
    jsonfile = CONTRACTS["TestICA"]
    tcontract = deploy_contract(ibc.cronos.w3, jsonfile)
    memo = {"src_callback": {"address": tcontract.address}}

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
        generated_tx = json.dumps(generated_tx_packet(msg_num))
        # submit transaction on host chain on behalf of interchain account
        rsp = cli_controller.ica_ctrl_send_tx(
            connid,
            generated_tx,
            gas=gas,
            from_="signer2",
        )
        assert rsp["code"] == 0, rsp["raw_log"]
        wait_for_check_tx(cli_host, ica_address, num_txs)

    msg_num = 10
    send_tx(msg_num)

    def check_for_ack():
        criteria = "message.action=/ibc.core.channel.v1.MsgAcknowledgement"
        return cli_controller.tx_search(criteria)["txs"]

    txs = wait_for_fn("ack change", check_for_ack)
    events = parse_events_rpc(txs[0]["events"])
    err = events.get("ibc_src_callback")["callback_error"]
    assert "sender is not authenticated" in err, err
