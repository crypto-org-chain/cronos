import shutil
import stat
import subprocess
from contextlib import contextmanager
from pathlib import Path

import pytest
from pystarport import ports

from .ibc_v18 import check_ibc_client_states, preupgrade_ibc_snapshot
from .network import Cronos, setup_custom_cronos
from .staking_v1_8 import postupgrade_check_staking, preupgrade_staking_setup
from .test_rollback import update_node_cmd
from .test_upgrade import check_basic_tx, post_init
from .upgrade_rehearsal import LoadGenerator, assert_no_divergence
from .utils import (
    ADDRS,
    KEYS,
    approve_proposal,
    send_transaction,
    supervisorctl,
    wait_for_block,
    wait_for_new_blocks,
    wait_for_port,
)

pytestmark = pytest.mark.upgrade

BASE_PORT = 26600
PLAN_NAME = "v1.8"


@pytest.fixture(scope="module")
def custom_cronos(tmp_path_factory):
    path = tmp_path_factory.mktemp("upgrade-rehearsal")
    cmd = [
        "nix-build",
        Path(__file__).parent / "configs/upgrade-rehearsal-package.nix",
    ]
    print(*cmd)
    subprocess.run(cmd, check=True, cwd=path)

    # copy the content so the new directory is writable.
    upgrades = path / "upgrades"
    shutil.copytree(path / "result", upgrades)
    mod = stat.S_IRWXU
    upgrades.chmod(mod)
    for d in upgrades.iterdir():
        d.chmod(mod)

    with contextmanager(setup_custom_cronos)(
        path,
        BASE_PORT,
        Path(__file__).parent / "configs/cosmovisor.jsonnet",
        post_init=post_init,
        chain_binary=str(upgrades / "genesis/bin/cronosd"),
    ) as cronos:
        yield cronos, upgrades


def rpc_ports(cronos: Cronos):
    return [
        ports.rpc_port(cronos.base_port(i))
        for i in range(len(cronos.config["validators"]))
    ]


def test_upgrade_rehearsal(custom_cronos):
    """
    Rehearse a cosmovisor in-place upgrade from v1.7.8-unsafe to v1.8 while
    load is running: assert the v1.8 migrations, that the chain doesn't
    halt, and that nodes don't diverge - then roll the upgrade back.
    """
    cronos, upgrades = custom_cronos
    cli = cronos.cosmos_cli()
    base_port = cronos.base_port(0)
    api_port = ports.api_port(base_port)

    staking_info = preupgrade_staking_setup(cli, cronos)
    ibc_snapshot = preupgrade_ibc_snapshot(api_port)
    preupgrade_height = cli.block_height()

    load = LoadGenerator(
        lambda: send_transaction(
            cronos.w3,
            {
                "to": ADDRS["community"],
                "value": 1000,
                "maxFeePerGas": 10000000000000,
                "maxPriorityFeePerGas": 10000,
            },
            key=KEYS["signer1"],
        )
    ).start()

    target_height = cli.block_height() + 15
    rsp = cli.submit_gov_proposal(
        "community",
        "software-upgrade",
        {
            "name": PLAN_NAME,
            "title": "upgrade rehearsal",
            "note": "ditto",
            "upgrade-height": target_height,
            "summary": "summary",
            "deposit": "10000basetcro",
        },
        broadcast_mode="sync",
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    approve_proposal(
        cronos, rsp["events"], msg="/cosmos.upgrade.v1beta1.MsgSoftwareUpgrade"
    )

    cronos.chain_binary = str(upgrades / f"{PLAN_NAME}/bin/cronosd")
    wait_for_block(cronos.cosmos_cli(), target_height + 2, timeout=480)
    wait_for_port(ports.evmrpc_port(base_port))
    cli = cronos.cosmos_cli()

    postupgrade_check_staking(cli, staking_info)
    check_ibc_client_states(api_port, ibc_snapshot)
    cronos_params = cli.query_params()
    assert cronos_params.get("cro_bridge_contract_addresses", []) == [
        "0x6b1b50c2223eb31E0d4683b046ea9C6CB0D0ea4F",
        "0xCE13a6F3d4167CE958f4764D423e6D62a114c751",
    ], f"unexpected cro_bridge_contract_addresses: {cronos_params}"

    # no halt: a few more blocks tick over and recent sends succeed.
    wait_for_new_blocks(cli, 3)
    check_basic_tx(cronos)
    assert load.results[-3:] == [True, True, True], load.results

    load.stop()
    postupgrade_height = cli.block_height()
    assert_no_divergence(rpc_ports(cronos), preupgrade_height, postupgrade_height)

    # rollback drill: undo every block produced on the v1.8 binary and switch
    # every node back to the pre-upgrade binary. --hard is required so each
    # call also removes the block from the blockstore, not just the state DB
    # height - otherwise a plain rollback() no-ops on the second call, and
    # CometBFT's replay-on-restart would just re-execute the discarded blocks.
    rollback_count = postupgrade_height - target_height + 1
    n = len(cronos.config["validators"])
    tasks_ini = cronos.base_dir / "../tasks.ini"
    genesis_binary = str(upgrades / "genesis/bin/cronosd")
    supervisorctl(tasks_ini, "stop", *(f"cronos_777-1-node{i}" for i in range(n)))
    for i in range(n):
        node_cli = cronos.cosmos_cli(i)
        for _ in range(rollback_count):
            node_cli.rollback(hard=True)
        update_node_cmd(cronos.base_dir, genesis_binary, i)
    supervisorctl(tasks_ini, "update")

    for i in range(n):
        wait_for_port(ports.rpc_port(cronos.base_port(i)))
    cronos.chain_binary = genesis_binary
    cli = cronos.cosmos_cli()
    wait_for_new_blocks(cli, 2)
    check_basic_tx(cronos)
    assert_no_divergence(rpc_ports(cronos), postupgrade_height, cli.block_height())
