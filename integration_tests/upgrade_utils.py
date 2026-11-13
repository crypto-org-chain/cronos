import json
import shutil
import stat
import subprocess
from pathlib import Path

from pystarport.cluster import SUPERVISOR_CONFIG_FILE

from .utils import approve_proposal, edit_ini_sections

CHAIN_ID = "cronos_777-1"


def prepare_upgrade_binaries(path, genesis="genesis", plans=None):
    """
    build the upgrade test package and copy it to path/upgrades, starting from
    the `genesis` release and keeping only `plans` (all of them by default)
    """
    pkg = Path(__file__).parent / "configs/upgrade-test-package.nix"
    print("nix-build", pkg)
    subprocess.run(["nix-build", pkg], check=True)

    # copy the content so the new directory is writable.
    result = Path("./result")
    upgrades = path / "upgrades"
    if genesis == "genesis" and plans is None:
        shutil.copytree(result, upgrades)
    else:
        upgrades.mkdir()
        shutil.copytree(result / genesis, upgrades / "genesis")
        for plan in plans or []:
            shutil.copytree(result / plan, upgrades / plan)
    mod = stat.S_IRWXU
    upgrades.chmod(mod)
    for d in upgrades.iterdir():
        d.chmod(mod)
    return upgrades


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
    data = path / CHAIN_ID
    cfg = json.loads((data / "config.json").read_text())
    for i, _ in enumerate(cfg["validators"]):
        home = data / f"node{i}"
        init_cosmovisor(home)

    edit_ini_sections(
        CHAIN_ID,
        data / SUPERVISOR_CONFIG_FILE,
        lambda i, _: {
            "command": f"cosmovisor run start --home %(here)s/node{i}",
            "environment": (
                "DAEMON_NAME=cronosd,"
                "DAEMON_SHUTDOWN_GRACE=1m,"
                "UNSAFE_SKIP_BACKUP=true,"
                f"DAEMON_HOME=%(here)s/node{i}"
            ),
        },
    )


def propose_software_upgrade(c, plan_name, target):
    rsp = c.cosmos_cli().submit_gov_proposal(
        "community",
        "software-upgrade",
        {
            "name": plan_name,
            "title": "upgrade test",
            "note": "ditto",
            "upgrade-height": target,
            "summary": "summary",
            "deposit": "10000basetcro",
        },
        broadcast_mode="sync",
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    approve_proposal(c, rsp["events"], msg="/cosmos.upgrade.v1beta1.MsgSoftwareUpgrade")
