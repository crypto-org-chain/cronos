import subprocess
import time

import pytest

from .ibc_utils import prepare_network

pytestmark = pytest.mark.ibc_update_client


@pytest.fixture(scope="module")
def ibc(request, tmp_path_factory):
    "prepare-network"
    name = "ibc"
    path = tmp_path_factory.mktemp(name)
    network = prepare_network(path, name)
    yield from network


def test_ibc_update_client(ibc):
    """
    test update expire subject client with new active client
    """
    cmd = [
        "hermes",
        "--config",
        ibc.hermes.configpath,
        "create",
        "client",
        "--host-chain",
        "cronos_777-1",
        "--reference-chain",
        "chainmain-1",
    ]
    subprocess.check_call(cmd + ["--trusting-period", "1s"])
    time.sleep(1)
    subprocess.check_call(cmd)
    cli = ibc.cronos.cosmos_cli()
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
