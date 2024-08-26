import time

import pytest

from .ibc_utils import (
    funds_ica,
    get_balance,
    ica_send_tx,
    prepare_network,
    register_acc,
)
from .utils import wait_for_fn

pytestmark = pytest.mark.ica


@pytest.fixture(scope="module")
def ibc(request, tmp_path_factory):
    "prepare-network"
    name = "ibc"
    path = tmp_path_factory.mktemp(name)
    yield from prepare_network(path, name)


def test_incentivized(ibc):
    connid = "connection-0"
    cli_host = ibc.chainmain.cosmos_cli()
    cli_controller = ibc.cronos.cosmos_cli()
    ica_address, channel_id = register_acc(cli_controller, connid)
    relayer = cli_controller.address("signer1")
    balance = funds_ica(cli_host, ica_address)
    ibc.cronos.supervisorctl("stop", "relayer-demo")
    time.sleep(3)
    port_id = "icahost"
    rsp = cli_host.register_counterparty_payee(
        port_id,
        channel_id,
        cli_host.address("relayer"),
        relayer,
        from_="relayer",
        fees="100000000basecro",
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    ibc.cronos.supervisorctl("start", "relayer-demo")
    to = cli_host.address("signer2")
    amount = 1000
    denom = "basecro"
    sender = cli_controller.address("signer2")
    fee_denom = "ibcfee"
    old_amt_fee = cli_controller.balance(relayer, fee_denom)
    old_amt_sender_fee = cli_controller.balance(sender, fee_denom)
    msg_num = 2
    fee = f"10{fee_denom}"

    def incentivized_cb(seq):
        rsp = cli_controller.pay_packet_fee(
            f"icacontroller-{sender}",
            channel_id,
            seq,
            recv_fee=fee,
            ack_fee=fee,
            timeout_fee=fee,
            from_=sender,
        )
        assert rsp["code"] == 0, rsp["raw_log"]

    ica_send_tx(
        cli_host,
        cli_controller,
        connid,
        ica_address,
        msg_num,
        to,
        denom,
        amount,
        fees="0basecro",
        incentivized_cb=incentivized_cb,
    )
    balance -= amount * msg_num

    # fee is locked
    # https://github.com/cosmos/ibc-go/pull/5571
    assert cli_controller.balance(sender, fee_denom) == old_amt_sender_fee - 20
    # check if the funds are reduced in interchain account
    assert cli_host.balance(ica_address, denom=denom) == balance

    # wait for relayer receive the fee
    def check_fee():
        amount = cli_controller.balance(relayer, fee_denom)
        if amount > old_amt_fee:
            assert amount == old_amt_fee + 20
            return True
        else:
            return False

    wait_for_fn("wait for relayer to receive the fee", check_fee)

    # timeout fee is refunded
    actual = get_balance(ibc.cronos, sender, fee_denom)
    assert actual == old_amt_sender_fee - 20, actual
