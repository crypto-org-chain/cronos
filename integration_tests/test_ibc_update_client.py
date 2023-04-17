import subprocess

import pytest
from dateutil.parser import isoparse

from .ibc_utils import prepare_network
from .utils import parse_events, wait_for_block_time, wait_for_fn


@pytest.fixture(scope="module")
def ibc(request, tmp_path_factory):
    "prepare-network"
    name = "ibc"
    path = tmp_path_factory.mktemp(name)
    network = prepare_network(path, name, True, False)
    yield from network


def test_ibc_update_client(ibc):
    """
    test update expire subject client with new active client
    """
    cli = ibc.cronos.cosmos_cli()
    cmd0 = ["hermes", "--config", ibc.hermes.configpath]
    # create new client with small trust in cronos
    cmd = cmd0 + [
        "create",
        "client",
        "--host-chain",
        "cronos_777-1",
        "--reference-chain",
        "chainmain-1",
    ]
    trust_period = "30s"
    subprocess.check_call(cmd + ["--trusting-period", trust_period])
    # create new connection with new client
    cmd = cmd0 + [
        "create",
        "connection",
        "--a-chain",
        "cronos_777-1",
        "--a-client",
        "07-tendermint-1",
        "--b-client",
        "07-tendermint-0",
    ]
    subprocess.check_call(cmd)
    # create new channel with new connection
    port_id = "transfer"
    cmd = cmd0 + [
        "create",
        "channel",
        "--a-chain",
        "cronos_777-1",
        "--a-connection",
        "connection-1",
        "--a-port",
        port_id,
        "--b-port",
        port_id,
    ]
    subprocess.check_call(cmd)

    def assert_trust_period(period):
        key = "trusting_period"
        res = cli.ibc_query_client_state(port_id, "channel-1")["client_state"][key]
        assert res == period, res

    assert_trust_period(trust_period)
    # create new client with default trust in cronos
    cmd = cmd0 + [
        "create",
        "client",
        "--host-chain",
        "cronos_777-1",
        "--reference-chain",
        "chainmain-1",
    ]
    subprocess.check_call(cmd)
    cmd = cmd0 + [
        "query",
        "client",
        "status",
        "--chain",
        "cronos_777-1",
        "--client",
        "07-tendermint-1",
    ]

    def check_status():
        raw = subprocess.check_output(cmd).decode("utf-8")
        status = raw.split()[1]
        return status != "Active"

    wait_for_fn("status change", check_status)
    rsp = cli.gov_propose_update_client_legacy(
        {
            "subject_client_id": "07-tendermint-1",
            "substitute_client_id": "07-tendermint-2",
            "from": "validator",
            "title": "update-client-title",
            "description": "update-client-description",
            "deposit": "1basetcro",
        },
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    # get proposal_id
    ev = parse_events(rsp["logs"])["submit_proposal"]
    proposal_id = ev["proposal_id"]
    rsp = cli.gov_vote("validator", proposal_id, "yes")
    assert rsp["code"] == 0, rsp["raw_log"]
    rsp = ibc.cronos.cosmos_cli(1).gov_vote("validator", proposal_id, "yes")
    assert rsp["code"] == 0, rsp["raw_log"]
    proposal = cli.query_proposal(proposal_id)
    wait_for_block_time(cli, isoparse(proposal["voting_end_time"]))
    proposal = cli.query_proposal(proposal_id)
    assert proposal["status"] == "PROPOSAL_STATUS_PASSED", proposal
    default_trust_period = "1209600s"
    assert_trust_period(default_trust_period)
