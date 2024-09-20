import asyncio
import json
import os
import shutil
import socket
import subprocess
import tarfile
import tempfile
import time
from pathlib import Path
from typing import List

import click
import tomlkit

from . import transaction
from .cli import ChainCommand
from .echo import run_echo_server
from .peer import (
    CONTAINER_CRONOSD_PATH,
    FULLNODE_GROUP,
    VALIDATOR_GROUP,
    gen_genesis,
    init_node,
    patch_configs,
)
from .stats import dump_block_stats
from .topology import connect_all
from .types import PeerPacket
from .utils import block_height, block_txs, wait_for_block, wait_for_port

# use cronosd on host machine
LOCAL_CRONOSD_PATH = "cronosd"
DEFAULT_CHAIN_ID = "cronos_777-1"
# the container must be deployed with the prefixed name
HOSTNAME_TEMPLATE = "testplan-{index}"
ECHO_SERVER_PORT = 26659


@click.group()
def cli():
    pass


def validate_json(ctx, param, value):
    try:
        return json.loads(value)
    except json.JSONDecodeError:
        raise click.BadParameter("must be a valid JSON string")


@cli.command()
@click.argument("outdir")
@click.option("--hostname-template", default=HOSTNAME_TEMPLATE)
@click.option("--validators", default=3)
@click.option("--fullnodes", default=7)
@click.option("--num-accounts", default=10)
@click.option("--num-txs", default=1000)
@click.option("--config-patch", default="{}", callback=validate_json)
@click.option("--app-patch", default="{}", callback=validate_json)
@click.option("--genesis-patch", default="{}", callback=validate_json)
@click.option("--validator-generate-load/--no-validator-generate-load", default=True)
def gen(**kwargs):
    return _gen(**kwargs)


@cli.command()
@click.argument("options", callback=validate_json)
def generic_gen(options: dict):
    return _gen(**options)


def _gen(
    outdir: str,
    hostname_template: str = HOSTNAME_TEMPLATE,
    validators: int = 3,
    fullnodes: int = 7,
    num_accounts: int = 10,
    num_txs: int = 1000,
    validator_generate_load: bool = True,
    config_patch: dict = None,
    app_patch: dict = None,
    genesis_patch: dict = None,
):
    config_patch = config_patch or {}
    app_patch = app_patch or {}
    genesis_patch = genesis_patch or {}

    outdir = Path(outdir)
    cli = ChainCommand(LOCAL_CRONOSD_PATH)
    (outdir / VALIDATOR_GROUP).mkdir(parents=True, exist_ok=True)
    (outdir / FULLNODE_GROUP).mkdir(parents=True, exist_ok=True)

    peers = []
    for i in range(validators):
        print("init validator", i)
        global_seq = i
        ip = hostname_template.format(index=global_seq)
        peers.append(
            init_node_local(
                cli, outdir, VALIDATOR_GROUP, i, global_seq, ip, num_accounts
            )
        )
    for i in range(fullnodes):
        print("init fullnode", i)
        global_seq = i + validators
        ip = hostname_template.format(index=global_seq)
        peers.append(
            init_node_local(
                cli, outdir, FULLNODE_GROUP, i, global_seq, ip, num_accounts
            )
        )

    print("prepare genesis")
    # use a full node directory to prepare the genesis file
    genesis = gen_genesis(cli, outdir / FULLNODE_GROUP / "0", peers, genesis_patch)

    print("patch genesis")
    # write genesis file and patch config files
    for i in range(validators):
        patch_configs_local(
            peers, genesis, outdir, VALIDATOR_GROUP, i, config_patch, app_patch
        )
    for i in range(fullnodes):
        patch_configs_local(
            peers,
            genesis,
            outdir,
            FULLNODE_GROUP,
            i,
            config_patch,
            app_patch,
        )

    print("write config")
    cfg = {
        "validators": validators,
        "fullnodes": fullnodes,
        "num_accounts": num_accounts,
        "num_txs": num_txs,
        "validator-generate-load": validator_generate_load,
    }
    (outdir / "config.json").write_text(json.dumps(cfg))


@cli.command()
@click.argument("toimage")
@click.argument("src")
@click.option("--dst", default="/data")
@click.option(
    "--fromimage", default="ghcr.io/crypto-org-chain/cronos-testground:latest"
)
def patchimage(toimage, src, dst, fromimage):
    """
    combine data directory with an exiting image to produce a new image
    """
    with tempfile.TemporaryDirectory() as tmpdir:
        tmpdir = Path(tmpdir)
        shutil.copytree(src, tmpdir / "out")
        content = f"""FROM {fromimage}
ADD ./out {dst}
"""
        print(content)
        (tmpdir / "Dockerfile").write_text(content)
        subprocess.run(["docker", "build", "-t", toimage, tmpdir])


@cli.command()
@click.option("--outdir", default="/outputs")
@click.option("--datadir", default="/data")
@click.option("--cronosd", default=CONTAINER_CRONOSD_PATH)
@click.option("--global-seq", default=None)
def run(outdir: str, datadir: str, cronosd, global_seq):
    datadir = Path(datadir)
    cfg = json.loads((datadir / "config.json").read_text())

    if global_seq is None:
        global_seq = node_index()

    validators = cfg["validators"]
    group = VALIDATOR_GROUP if global_seq < validators else FULLNODE_GROUP
    group_seq = global_seq if group == VALIDATOR_GROUP else global_seq - validators
    print("node role", global_seq, group, group_seq)
    home = datadir / group / str(group_seq)

    try:
        return do_run(datadir, home, cronosd, group, global_seq, cfg)
    finally:
        # collect outputs
        output = Path("/data.tar.bz2")
        with tarfile.open(output, "x:bz2") as tar:
            tar.add(home, arcname="data", filter=output_filter(group, group_seq))
        outdir = Path(outdir)
        if outdir.exists():
            assert outdir.is_dir()
            filename = outdir / f"{group}_{group_seq}.tar.bz2"
            filename.unlink(missing_ok=True)
            shutil.copy(output, filename)


@cli.command()
@click.argument("outdir")
@click.option("--nodes", default=10)
@click.option("--num-accounts", default=10)
@click.option("--num-txs", default=1000)
def gen_txs(**kwargs):
    return _gen_txs(**kwargs)


@cli.command()
@click.argument("options", callback=validate_json)
def generic_gen_txs(options: dict):
    return _gen_txs(**options)


def _gen_txs(
    outdir: str,
    nodes: int = 10,
    num_accounts: int = 10,
    num_txs: int = 1000,
):
    outdir = Path(outdir)
    for global_seq in range(nodes):
        print("generating", num_accounts * num_txs, "txs for node", global_seq)
        txs = transaction.gen(global_seq, num_accounts, num_txs)
        transaction.save(txs, outdir, global_seq)
        print("saved", len(txs), "txs for node", global_seq)


def do_run(
    datadir: Path, home: Path, cronosd: str, group: str, global_seq: int, cfg: dict
):
    if group == FULLNODE_GROUP or cfg.get("validator-generate-load", True):
        txs = transaction.load(datadir, global_seq)
        if txs:
            print("loaded", len(txs), "txs")
        else:
            print(
                "generating",
                cfg["num_accounts"] * cfg["num_txs"],
                "txs for node",
                global_seq,
            )
            txs = transaction.gen(global_seq, cfg["num_accounts"], cfg["num_txs"])
    else:
        txs = []

    # wait for persistent peers to be ready
    run_echo_server(ECHO_SERVER_PORT)
    wait_for_peers(home)

    print("start node")
    logfile = open(home / "node.log", "ab", buffering=0)
    proc = subprocess.Popen(
        [cronosd, "start", "--home", str(home)],
        stdout=logfile,
    )

    cli = ChainCommand(cronosd)
    wait_for_port(26657)
    wait_for_port(8545)
    wait_for_block(cli, 3)

    if txs:
        asyncio.run(transaction.send(txs))

    # node quit when the chain is idle or halted for a while
    detect_idle_halted(20, 20)

    with (home / "block_stats.log").open("w") as logfile:
        dump_block_stats(logfile)

    proc.kill()
    proc.wait(20)


def output_filter(group, group_seq: int):
    """
    filter out some big and useless paths to reduce size of output artifacts
    """

    is_validator_leader = group == VALIDATOR_GROUP and group_seq == 0
    is_fullnode_leader = group == FULLNODE_GROUP and group_seq == 0

    def inner(info: tarfile.TarInfo):
        # only keep one copy
        if not is_validator_leader and info.name in (
            "data/data/cs.wal",
            "data/data/blockstore.db",
            "data/data/application.db",
            "data/data/memiavl.db",
            "data/data/state.db",
        ):
            return None
        if not is_fullnode_leader and info.name == "data/data/tx_index.db":
            return None
        return info

    return inner


def detect_idle_halted(idle_blocks: int, interval: int, chain_halt_interval=120):
    """
    returns if the chain is empty for consecutive idle_blocks, or halted.

    idle_blocks: the number of consecutive empty blocks to check
    interval: poll interval
    chain_halt_interval: the chain is considered halted if no new block for this time
    """
    last_time = time.time()
    prev_height = 0

    while True:
        latest = block_height()
        if latest > prev_height:
            prev_height = latest
            last_time = time.time()

            # detect chain idle if made progress
            print("current block", latest)
            for i in range(idle_blocks):
                height = latest - i
                if height <= 0:
                    break
                if len(block_txs(height)) > 0:
                    break
            else:
                # normal quit means idle
                return

            # break early means not idle
            time.sleep(interval)
            continue
        else:
            # detect chain halt if no progress
            if time.time() - last_time >= chain_halt_interval:
                print(f"chain didn't make progress for {chain_halt_interval} seconds")
                return


def init_node_local(
    cli: ChainCommand,
    outdir: Path,
    group: str,
    group_seq: int,
    global_seq: int,
    ip: str,
    num_accounts: int,
) -> PeerPacket:
    return init_node(
        cli,
        outdir / group / str(group_seq),
        ip,
        DEFAULT_CHAIN_ID,
        group,
        group_seq,
        global_seq,
        num_accounts=num_accounts,
    )


def patch_configs_local(
    peers: List[PeerPacket],
    genesis,
    outdir: Path,
    group: str,
    i: int,
    config_patch,
    app_patch,
):
    home = outdir / group / str(i)
    (home / "config" / "genesis.json").write_text(json.dumps(genesis))
    p2p_peers = connect_all(peers[i], peers)
    patch_configs(home, p2p_peers, config_patch, app_patch)


def node_index() -> int:
    """
    1. try indexed job in k8s,
       see: https://kubernetes.io/docs/tasks/job/indexed-parallel-processing-static/
    2. try hostname
    """
    i = os.environ.get("JOB_COMPLETION_INDEX")
    if i is not None:
        return int(i)
    hostname = socket.gethostname()
    return int(hostname.rsplit("-", 1)[-1])


def wait_for_peers(home: Path):
    cfg = tomlkit.parse((home / "config" / "config.toml").read_text())
    peers = cfg["p2p"]["persistent_peers"]
    for peer in peers.split(","):
        host = peer.split("@", 1)[1].split(":", 1)[0]
        print("wait for peer to be ready:", host)
        wait_for_port(ECHO_SERVER_PORT, host=host, timeout=2400)


if __name__ == "__main__":
    cli()
