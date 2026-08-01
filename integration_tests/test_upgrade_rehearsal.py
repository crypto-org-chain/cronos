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
# Safety bound on the rollback loop: the drill undoes the ~20 blocks produced
# on the v1.8 binary plus the ~15 that separate the proposal from the upgrade
# height, so anything near this means rollback stopped making progress.
MAX_ROLLBACKS = 200
# Receipt wait for the background load: a few blocks' worth, so a send that
# lands during the upgrade outage fails fast instead of pinning the worker.
RECEIPT_TIMEOUT = 5


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
            # bounded well under LoadGenerator.stop()'s join timeout: web3's
            # 120s default would leave the worker stuck through the upgrade
            # outage and stop() would give up on it
            timeout=RECEIPT_TIMEOUT,
        )
    ).start()

    # No block at or below this height carries the upgrade proposal, so it is
    # also the floor the rollback drill below has to reach.
    preproposal_height = cli.block_height()
    target_height = preproposal_height + 15
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
    # Baseline for the rollback drill: the plan is in state now, at a height the
    # chain has not reached yet. Without this the later "plan is gone" checks
    # could pass on a chain that was never scheduled to upgrade at all.
    plan = cli.query_upgrade_plan()
    assert plan.get("name") == PLAN_NAME, plan
    assert int(plan["height"]) == target_height, plan
    assert cli.query_upgrade_applied(PLAN_NAME) == 0, "v1.8 already applied"

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
    # x/upgrade consumed the plan and wrote a done record: this pair is exactly
    # what the rollback drill below has to undo, and it is readable on both
    # binaries (unlike cro_bridge_contract_addresses, a field the pre-upgrade
    # binary's Params message does not even have).
    assert cli.query_upgrade_plan() == {}, "plan still scheduled after the upgrade"
    assert cli.query_upgrade_applied(PLAN_NAME) == target_height

    # no halt: a few more blocks tick over and recent sends succeed.
    wait_for_new_blocks(cli, 3)
    check_basic_tx(cronos)
    assert load.results[-3:] == [True, True, True], load.results

    load.stop()
    # A node whose v1.8 execution diverged from the rest would have panicked at
    # the upgrade height and stopped answering, so watching the post-upgrade
    # blocks catches it: every node has to report its own app hash here.
    assert_no_divergence(rpc_ports(cronos), blocks=3)

    # rollback drill: undo every block produced on the v1.8 binary and switch
    # every node back to the pre-upgrade binary. --hard is required so each
    # call also removes the block from the blockstore, not just the state DB
    # height - otherwise a plain rollback() no-ops on the second call, and
    # CometBFT's replay-on-restart would just re-execute the discarded blocks.
    #
    # The floor is preproposal_height, not target_height - 1: x/upgrade stores
    # the plan when the proposal passes, well before the upgrade height, so
    # stopping at target_height - 1 leaves the v1.8 plan in state and the
    # pre-upgrade binary panics with `UPGRADE "v1.8" NEEDED` the moment it
    # reaches target_height again. Undoing the proposal itself is also what an
    # operator has to do to get a chain running on the old binary again.
    #
    # Each node's tip is only known once it has stopped: the divergence check
    # above spends dozens of HTTP round-trips while blocks keep committing, so
    # any height read before it is stale, and a rollback count derived from it
    # would leave v1.8 blocks in the store of a node restarted on v1.7.8.
    # rollback() reports the height it left behind, so drive the loop off that.
    n = len(cronos.config["validators"])
    tasks_ini = cronos.base_dir / "../tasks.ini"
    genesis_binary = str(upgrades / "genesis/bin/cronosd")
    supervisorctl(tasks_ini, "stop", *(f"cronos_777-1-node{i}" for i in range(n)))
    for i in range(n):
        node_cli = cronos.cosmos_cli(i)
        for _ in range(MAX_ROLLBACKS):
            if node_cli.rollback(hard=True) <= preproposal_height:
                break
        else:
            raise AssertionError(
                f"node{i} still above rollback point {preproposal_height} after "
                f"{MAX_ROLLBACKS} rollbacks"
            )
        update_node_cmd(cronos.base_dir, genesis_binary, i)
    supervisorctl(tasks_ini, "update")

    for i in range(n):
        wait_for_port(ports.rpc_port(cronos.base_port(i)))
    cronos.chain_binary = genesis_binary
    cli = cronos.cosmos_cli()
    wait_for_new_blocks(cli, 2)
    check_basic_tx(cronos)
    # The tip alone proves nothing here - the wait above already advanced it.
    # What makes this a rollback is that the v1.8 upgrade is undone in state:
    # the done record is gone (a rollback that never happened would keep it) and
    # no plan is scheduled again (a rollback that stopped between the proposal
    # and target_height would leave the plan, and the pre-upgrade binary would
    # panic with `UPGRADE "v1.8" NEEDED` on reaching that height).
    postrollback_height = cli.block_height()
    assert cli.query_upgrade_applied(PLAN_NAME) == 0, (
        f"v1.8 still recorded as applied after rollback (tip {postrollback_height}, "
        f"rollback point {preproposal_height})"
    )
    assert cli.query_upgrade_plan() == {}, "v1.8 still scheduled after rollback"
    # And the nodes really are back on the pre-upgrade binary: its cronos Params
    # message has no cro_bridge_contract_addresses field at all (added in v1.8),
    # while the v1.8 binary always renders the key, empty list included.
    rolled_back_params = cli.query_params()
    assert "cro_bridge_contract_addresses" not in rolled_back_params, (
        f"node is still answering from the v1.8 binary: {rolled_back_params}"
    )
    # Every node re-produces the rolled-back heights on the pre-upgrade binary;
    # if any of them replayed to a different state it panics rather than commit,
    # which shows up as a node that never reports an app hash.
    assert_no_divergence(rpc_ports(cronos), blocks=3)
