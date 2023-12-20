import configparser
import subprocess
from pathlib import Path

import pytest
from pystarport import ports
from pystarport.cluster import SUPERVISOR_CONFIG_FILE

from .network import setup_custom_cronos
from .utils import supervisorctl, wait_for_block, wait_for_port

pytestmark = pytest.mark.slow


def update_node2_cmd(path, cmd, i):
    ini_path = path / SUPERVISOR_CONFIG_FILE
    ini = configparser.RawConfigParser()
    ini.read(ini_path)
    for section in ini.sections():
        if section == f"program:cronos_777-1-node{i}":
            ini[section].update(
                {
                    "command": f"{cmd} start --home %(here)s/node{i}",
                    "autorestart": "false",  # don't restart when stopped
                }
            )
    with ini_path.open("w") as fp:
        ini.write(fp)


def post_init(broken_binary):
    def inner(path, base_port, config):
        chain_id = "cronos_777-1"
        update_node2_cmd(path / chain_id, broken_binary, 2)

    return inner


@pytest.fixture(scope="module")
def custom_cronos(tmp_path_factory):
    path = tmp_path_factory.mktemp("rollback")

    cmd = [
        "nix-build",
        "--no-out-link",
        Path(__file__).parent / "configs/broken-cronosd.nix",
    ]
    print(*cmd)
    broken_binary = Path(subprocess.check_output(cmd).strip().decode()) / "bin/cronosd"
    print(broken_binary)

    # init with genesis binary
    yield from setup_custom_cronos(
        path,
        26400,
        Path(__file__).parent / "configs/rollback.jsonnet",
        post_init=post_init(broken_binary),
        wait_port=False,
    )


def test_rollback(custom_cronos):
    """
    test using rollback command to fix app-hash mismatch situation.
    - the broken node will sync up to block 10 then crash.
    - use rollback command to rollback the db.
    - switch to correct binary should make the node syncing again.
    """
    wait_for_port(ports.rpc_port(custom_cronos.base_port(2)))

    print("wait for node2 to sync the first 10 blocks")
    cli2 = custom_cronos.cosmos_cli(2)
    wait_for_block(cli2, 10)

    print("wait for a few more blocks on the healthy nodes")
    cli = custom_cronos.cosmos_cli(0)
    wait_for_block(cli, 13)

    # (app hash mismatch happens after the 10th block, detected in the 11th block)
    print("check node2 get stuck at block 10")
    assert cli2.block_height() == 10

    print("stop node2")
    supervisorctl(custom_cronos.base_dir / "../tasks.ini", "stop", "cronos_777-1-node2")

    print("do rollback on node2")
    cli2.rollback()

    print("switch to normal binary")
    update_node2_cmd(custom_cronos.base_dir, "cronosd", 2)
    supervisorctl(custom_cronos.base_dir / "../tasks.ini", "update")
    wait_for_port(ports.rpc_port(custom_cronos.base_port(2)))

    print("check node2 sync again")
    cli2 = custom_cronos.cosmos_cli(2)
    wait_for_block(cli2, 15)
