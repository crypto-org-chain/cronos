import json
import subprocess
from pathlib import Path

import pytest
from dateutil.parser import isoparse
from pystarport import ports
from pystarport.cluster import SUPERVISOR_CONFIG_FILE

from .network import Cronos, setup_custom_cronos
from .utils import (
    edit_ini_sections,
    parse_events,
    wait_for_block,
    wait_for_block_time,
    wait_for_port,
)


def init_cosmovisor(home):
    """
    build and setup cosmovisor directory structure in each node's home directory
    """
    cosmovisor = home / "cosmovisor"
    cosmovisor.mkdir()
    (cosmovisor / "upgrades").symlink_to("../../../upgrades")
    (cosmovisor / "genesis").symlink_to("./upgrades/genesis")


def post_init(path, base_port, config):
    """
    prepare cosmovisor for each node
    """
    chain_id = "cronos_777-1"
    data = path / chain_id
    cfg = json.loads((data / "config.json").read_text())
    for i, _ in enumerate(cfg["validators"]):
        home = data / f"node{i}"
        init_cosmovisor(home)

    edit_ini_sections(
        chain_id,
        data / SUPERVISOR_CONFIG_FILE,
        lambda i, _: {
            "command": f"cosmovisor start --home %(here)s/node{i}"
            f" --trace --unsafe-experimental",
            "environment": f"DAEMON_NAME=cronosd,DAEMON_HOME=%(here)s/node{i}",
        },
    )


@pytest.fixture(scope="module")
def custom_cronos(tmp_path_factory):
    path = tmp_path_factory.mktemp("upgrade")
    cmd = [
        "nix-build",
        Path(__file__).parent / "configs/upgrade-test-package-gravity.nix",
        "-o",
        path / "upgrades",
    ]
    print(*cmd)
    subprocess.run(cmd, check=True)
    # init with genesis binary
    yield from setup_custom_cronos(
        path,
        26100,
        Path(__file__).parent / "configs/cosmovisor_gravity.jsonnet",
        post_init=post_init,
        chain_binary=str(path / "upgrades/genesis/bin/cronosd"),
    )


@pytest.mark.skip(reason="gravity-bridge not supported in v1.0.x")
def test_cosmovisor_upgrade_gravity(custom_cronos: Cronos):
    """
    - propose an upgrade and pass it
    - wait for it to happen
    - it should work transparently
    """
    cli = custom_cronos.cosmos_cli()
    height = cli.block_height()
    target_height = height + 15
    print("upgrade height", target_height)
    plan_name = "v0.8.0-gravity-alpha3"
    rsp = cli.gov_propose_v0_7(
        "community",
        "software-upgrade",
        {
            "name": plan_name,
            "title": "upgrade test",
            "description": "ditto",
            "upgrade-height": target_height,
            "deposit": "10000basetcro",
        },
    )
    assert rsp["code"] == 0, rsp["raw_log"]

    # get proposal_id
    ev = parse_events(rsp["logs"])["submit_proposal"]
    assert ev["proposal_type"] == "SoftwareUpgrade", rsp
    proposal_id = ev["proposal_id"]

    rsp = cli.gov_vote("validator", proposal_id, "yes")
    assert rsp["code"] == 0, rsp["raw_log"]
    rsp = custom_cronos.cosmos_cli(1).gov_vote("validator", proposal_id, "yes")
    assert rsp["code"] == 0, rsp["raw_log"]

    proposal = cli.query_proposal(proposal_id)
    wait_for_block_time(cli, isoparse(proposal["voting_end_time"]))
    proposal = cli.query_proposal(proposal_id)
    assert proposal["status"] == "PROPOSAL_STATUS_PASSED", proposal

    # update cli chain binary
    custom_cronos.chain_binary = (
        Path(custom_cronos.chain_binary).parent.parent.parent
        / f"{plan_name}/bin/cronosd"
    )
    cli = custom_cronos.cosmos_cli()

    # block should pass the target height
    wait_for_block(cli, target_height + 2, timeout=480)
    wait_for_port(ports.rpc_port(custom_cronos.base_port(0)))

    # check ica controller is enabled
    assert cli.query_gravity_params() == {
        "params": {
            "gravity_id": "defaultgravityid",
            "contract_source_hash": "",
            "bridge_ethereum_address": "0x0000000000000000000000000000000000000000",
            "bridge_chain_id": "0",
            "signed_signer_set_txs_window": "10000",
            "signed_batches_window": "10000",
            "ethereum_signatures_window": "10000",
            "target_eth_tx_timeout": "43200000",
            "average_block_time": "5000",
            "average_ethereum_block_time": "15000",
            "slash_fraction_signer_set_tx": "0.001000000000000000",
            "slash_fraction_batch": "0.001000000000000000",
            "slash_fraction_ethereum_signature": "0.001000000000000000",
            "slash_fraction_conflicting_ethereum_signature": "0.001000000000000000",
            "unbond_slashing_signer_set_txs_window": "10000",
            "bridge_active": True,
            "batch_creation_period": "10",
            "batch_max_element": "100",
            "observe_ethereum_height_period": "50",
        }
    }
