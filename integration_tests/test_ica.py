import json

import pytest
from pystarport import cluster

from .ibc_utils import (
    funds_ica,
    gen_send_msg,
    prepare_network,
    register_acc,
    wait_for_check_channel_ready,
    wait_for_check_tx,
)


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
    num_txs = len(cli_host.query_all_txs(ica_address)["txs"])
    to = cli_host.address("signer2")
    amount = 1000
    denom = "basecro"

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
    # check if the funds are reduced in interchain account
    assert cli_host.balance(ica_address, denom=denom) == balance - amount * msg_num
