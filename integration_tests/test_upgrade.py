import configparser
import json
import re
import subprocess
from pathlib import Path

import pytest
from dateutil.parser import isoparse
from pystarport import ports
from pystarport.cluster import SUPERVISOR_CONFIG_FILE

from .network import Cronos, setup_custom_cronos
from .utils import parse_events, wait_for_block, wait_for_block_time, wait_for_port


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
    cfg = json.loads((path / chain_id / "config.json").read_text())
    for i, _ in enumerate(cfg["validators"]):
        home = path / chain_id / f"node{i}"
        init_cosmovisor(home)

    # patch supervisord ini config
    ini_path = path / chain_id / SUPERVISOR_CONFIG_FILE
    ini = configparser.RawConfigParser()
    ini.read(ini_path)
    reg = re.compile(rf"^program:{chain_id}-node(\d+)")
    for section in ini.sections():
        m = reg.match(section)
        if m:
            i = m.group(1)
            ini[section].update(
                {
                    "command": f"cosmovisor start --home %(here)s/node{i}",
                    "environment": f"DAEMON_NAME=cronosd,DAEMON_HOME=%(here)s/node{i}",
                }
            )
    with ini_path.open("w") as fp:
        ini.write(fp)


@pytest.fixture(scope="module")
def custom_cronos(tmp_path_factory):
    path = tmp_path_factory.mktemp("upgrade")
    cmd = [
        "nix-build",
        Path(__file__).parent / "configs/upgrade-test-package.nix",
        "-o",
        path / "upgrades",
    ]
    print(*cmd)
    subprocess.run(cmd, check=True)
    # init with genesis binary
    yield from setup_custom_cronos(
        path,
        26100,
        Path(__file__).parent / "configs/cosmovisor.yaml",
        post_init=post_init,
        chain_binary=str(path / "upgrades/genesis/bin/cronosd"),
    )


def test_cosmovisor_upgrade(custom_cronos: Cronos):
    """
    - propose an upgrade and pass it
    - wait for it to happen
    - it should work transparently
    """
    cli = custom_cronos.cosmos_cli()
    height = cli.block_height()
    target_height = height + 15
    print("upgrade height", target_height)
    plan_name = "v0.7.0"
    rsp = cli.gov_propose(
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

    # block should pass the target height
    wait_for_block(cli, target_height + 2, timeout=480)

    # check feemarket is enabled correctly
    wait_for_port(ports.evmrpc_port(custom_cronos.base_port(0)))
    w3 = custom_cronos.w3
    # check base fee values
    fee1 = 5000000000000 - 5000000000000 // 100000000
    fee2 = fee1 - fee1 // 100000000
    assert w3.eth.get_block(target_height).baseFeePerGas == fee1
    assert w3.eth.get_block(target_height + 1).baseFeePerGas == fee2

    # query legacy blocks before the upgrade
    assert "baseFeePerGas" not in w3.eth.get_block(target_height - 1)
