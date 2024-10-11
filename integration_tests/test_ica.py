import base64
import json

import cprotobuf
import pytest
from pystarport import cluster

from .cosmoscli import module_address
from .ibc_utils import (
    ChannelOrder,
    Status,
    deploy_contract,
    funds_ica,
    gen_send_msg,
    ica_send_tx,
    parse_events_rpc,
    prepare_network,
    register_acc,
    wait_for_check_channel_ready,
    wait_for_check_tx,
    wait_for_status_change,
)
from .utils import CONTRACTS, approve_proposal, wait_for_fn

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
        relayer=cluster.Relayer.HERMES.value,
    )


@pytest.mark.parametrize(
    "order", [ChannelOrder.ORDERED.value, ChannelOrder.UNORDERED.value]
)
def test_ica(ibc, order, tmp_path):
    signer = "signer2" if order == ChannelOrder.ORDERED.value else "community"
    connid = "connection-0"
    cli_host = ibc.chainmain.cosmos_cli()
    cli_controller = ibc.cronos.cosmos_cli()
    ica_address, _, channel_id = register_acc(
        cli_controller, connid, ordering=order, signer=signer
    )
    balance = funds_ica(cli_host, ica_address, signer=signer)
    to = cli_host.address(signer)
    amount = 1000
    denom = "basecro"
    jsonfile = CONTRACTS["TestICA"]
    tcontract = deploy_contract(ibc.cronos.w3, jsonfile)
    timeout_in_ns = 6000000000
    seq = 1
    msg_num = 10
    assert tcontract.caller.getStatus(channel_id, seq) == Status.PENDING
    res = ica_send_tx(
        cli_host,
        cli_controller,
        connid,
        ica_address,
        msg_num,
        to,
        denom,
        amount,
        memo={"src_callback": {"address": tcontract.address}},
        signer=signer,
    )
    assert res == seq, res
    balance -= amount * msg_num
    assert cli_host.balance(ica_address, denom=denom) == balance
    wait_for_status_change(tcontract, channel_id, seq, timeout_in_ns / 1e9)
    assert tcontract.caller.getStatus(channel_id, seq) == Status.PENDING

    def check_for_ack():
        criteria = "message.action='/ibc.core.channel.v1.MsgAcknowledgement'"
        return cli_controller.tx_search(criteria)["txs"]

    txs = wait_for_fn("ack change", check_for_ack)
    events = parse_events_rpc(txs[0]["events"])
    err = events.get("ibc_src_callback")["callback_error"]
    assert "sender is not authenticated" in err, err

    no_timeout = 60

    def submit_msgs(msg_num, timeout_in_s=no_timeout, gas="200000"):
        num_txs = len(cli_host.query_all_txs(ica_address)["txs"])
        # generate a transaction to send to host chain
        m = gen_send_msg(ica_address, to, denom, amount)
        msgs = []
        for i in range(msg_num):
            msgs.append(m)
        data = json.dumps(msgs)
        packet = cli_controller.ica_generate_packet_data(data)
        # submit transaction on host chain on behalf of interchain account
        rsp = cli_controller.ica_send_tx(
            connid,
            json.dumps(packet),
            timeout_in_ns=int(timeout_in_s * 1e9),
            gas=gas,
            from_=signer,
        )
        assert rsp["code"] == 0, rsp["raw_log"]
        timeout = timeout_in_s + 3 if timeout_in_s < no_timeout else None
        wait_for_check_tx(cli_host, ica_address, num_txs, timeout)

    # submit large txs to trigger close channel with small timeout for order channel
    msg_num = 140
    submit_msgs(msg_num, 0.005, "600000")
    assert cli_host.balance(ica_address, denom=denom) == balance
    if order == ChannelOrder.UNORDERED.value:
        with pytest.raises(AssertionError) as exc:
            register_acc(cli_controller, connid)
        assert "existing active channel" in str(exc.value)
    else:
        wait_for_check_channel_ready(cli_controller, connid, channel_id, "STATE_CLOSED")
        # reopen ica account after channel get closed
        ica_address2, port_id2, channel_id2 = register_acc(cli_controller, connid)
        assert ica_address2 == ica_address, ica_address2
        assert channel_id2 != channel_id, channel_id2
        # upgrade to unordered channel
        channel = cli_controller.ibc_query_channel(port_id2, channel_id2)
        version_data = json.loads(channel["channel"]["version"])
        community = "community"
        authority = module_address("gov")
        deposit = "1basetcro"
        proposal_src = cli_controller.ibc_upgrade_channels(
            json.loads(version_data["app_version"]),
            community,
            deposit=deposit,
            title="channel-upgrade-title",
            summary="summary",
            port_pattern=port_id2,
            channel_ids=channel_id2,
        )
        proposal_src["deposit"] = deposit
        proposal_src["proposer"] = authority
        proposal_src["messages"][0]["signer"] = authority
        proposal_src["messages"][0]["fields"]["ordering"] = ChannelOrder.UNORDERED.value
        proposal = tmp_path / "proposal.json"
        proposal.write_text(json.dumps(proposal_src))
        rsp = cli_controller.submit_gov_proposal(proposal, from_=community)
        assert rsp["code"] == 0, rsp["raw_log"]
        approve_proposal(ibc.cronos, rsp["events"])
        wait_for_check_channel_ready(
            cli_controller, connid, channel_id2, "STATE_FLUSHCOMPLETE"
        )
        wait_for_check_channel_ready(cli_controller, connid, channel_id2)
        # submit large txs to trigger close channel with small timeout for order channel
        msg_num = 140
        submit_msgs(msg_num, 0.005, "600000")
        assert cli_host.balance(ica_address, denom=denom) == balance
        with pytest.raises(AssertionError) as exc:
            register_acc(cli_controller, connid)
        assert "existing active channel" in str(exc.value)

    # submit normal txs should work
    msg_num = 2
    submit_msgs(msg_num)
    balance -= amount * msg_num
    # check if the funds are reduced in interchain account
    assert cli_host.balance(ica_address, denom=denom) == balance


class QueryBalanceRequest(cprotobuf.Entity):
    address = cprotobuf.Field("string", 1)
    denom = cprotobuf.Field("string", 2)


def test_module_safe_query(ibc, tmp_path):
    cli_controller = ibc.cronos.cosmos_cli()
    signer = cli_controller.address("community")
    connid = "connection-0"
    query = QueryBalanceRequest(address=signer, denom="basecro")
    data = json.dumps(
        {
            "@type": "/ibc.applications.interchain_accounts.host.v1.MsgModuleQuerySafe",
            "signer": signer,
            "requests": [
                {
                    "path": "/cosmos.bank.v1beta1.Query/Balance",
                    "data": base64.b64encode(query.SerializeToString()),
                }
            ],
        }
    )
    packet = cli_controller.ica_generate_packet_data(data)
    timeout = 60  # in seconds
    cli_controller.ica_send_tx(
        connid,
        json.dumps(packet),
        timeout_in_ns=int(timeout * 1e9),
        gas=200000,
        from_=signer,
    )
