import json
import subprocess

import pytest

from .ibc_utils import prepare_network
from .utils import approve_proposal, wait_for_fn

pytestmark = pytest.mark.ibc_update_client


@pytest.fixture(scope="module")
def ibc(request, tmp_path_factory):
    "prepare-network"
    name = "ibc"
    path = tmp_path_factory.mktemp(name)
    yield from prepare_network(path, name, is_relay=False)


def test_ibc_update_client(ibc, tmp_path):
    """
    test client via chain header
    """
    cli = ibc.cronos.cosmos_cli()
    cli1 = ibc.chainmain.cosmos_cli()
    client_id = "07-tendermint-0"
    state = cli.ibc_query_client_consensus_states(client_id)["consensus_states"]
    trusted_height = state[-1]["height"]
    h = int(trusted_height["revision_height"])
    validators = cli1.ibc_query_client_header(h)["validator_set"]
    header = cli1.ibc_query_client_header(h + 1)
    header["trusted_validators"] = validators
    header["trusted_height"] = trusted_height
    header_json = header | {
        "@type": "/ibc.lightclients.tendermint.v1.Header",
    }
    header_file = tmp_path / "header.json"
    header_file.write_text(json.dumps(header_json))
    rsp = cli.ibc_update_client_with_header(client_id, header_file, from_="community")
    assert rsp["code"] == 0, rsp["raw_log"]


def test_ibc_update_client_via_proposal(ibc):
    """
    test update expire subject client with new active client via proposal
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
    trust_period = "45s"
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
    rsp = cli.ibc_recover_client(
        {
            "subject_client_id": "07-tendermint-1",
            "substitute_client_id": "07-tendermint-2",
            "from": "validator",
            "title": "update-client-title",
            "summary": "summary",
            "deposit": "1basetcro",
        },
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    approve_proposal(ibc.cronos, rsp)
    default_trust_period = "1209600s"
    assert_trust_period(default_trust_period)
