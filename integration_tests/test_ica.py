import json

import pytest

from .ibc_utils import (
    assert_channel_open_init,
    funds_ica,
    gen_send_msg,
    prepare_network,
    wait_for_check_channel_ready,
    wait_for_check_tx,
)


@pytest.fixture(scope="module")
def ibc(request, tmp_path_factory):
    "prepare-network"
    name = "ibc"
    path = tmp_path_factory.mktemp(name)
    network = prepare_network(path, name, incentivized=False, connection_only=True)
    yield from network


def test_ica(ibc, tmp_path):
    connid = "connection-0"
    cli_host = ibc.chainmain.cosmos_cli()
    cli_controller = ibc.cronos.cosmos_cli()

    print("register ica account")
    rsp = cli_controller.icaauth_register_account(
        connid, from_="signer2", gas="400000", fees="100000000basetcro"
    )
    _, channel_id = assert_channel_open_init(rsp)
    wait_for_check_channel_ready(cli_controller, connid, channel_id)

    print("query ica account")
    ica_address = cli_controller.ica_query_account(
        connid, cli_controller.address("signer2")
    )["interchain_account_address"]
    print("ica address", ica_address)

    balance = funds_ica(cli_host, ica_address)
    num_txs = len(cli_host.query_all_txs(ica_address)["txs"])

    # generate a transaction to send to host chain
    generated_tx = tmp_path / "generated_tx.txt"
    to = cli_host.address("signer2")
    amount = 1000
    denom = "basecro"
    # generate msgs send to host chain
    m = gen_send_msg(ica_address, to, denom, amount)
    msgs = []
    for i in range(2):
        msgs.append(m)
        balance -= amount
    fee = {"denom": "basetcro", "amount": "20000000000000000"}
    generated_tx_msg = {
        "body": {
            "messages": msgs,
        },
        "auth_info": {
            "fee": {
                "amount": [fee],
                "gas_limit": "200000",
            },
        },
    }
    generated_tx.write_text(json.dumps(generated_tx_msg))
    # submit transaction on host chain on behalf of interchain account
    rsp = cli_controller.icaauth_submit_tx(
        connid,
        generated_tx,
        timeout_duration="2h",
        from_="signer2",
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    packet_seq = next(
        int(evt["attributes"][4]["value"])
        for evt in rsp["events"]
        if evt["type"] == "send_packet"
    )
    print("packet sequence", packet_seq)
    wait_for_check_tx(cli_host, ica_address, num_txs)
    # check if the funds are reduced in interchain account
    assert cli_host.balance(ica_address, denom=denom) == balance
