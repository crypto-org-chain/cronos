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

import fire
import requests
import tomlkit

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
from .sendtx import generate_load
from .topology import connect_all
from .types import PeerPacket
from .utils import wait_for_block, wait_for_port, wait_for_w3

# use cronosd on host machine
LOCAL_CRONOSD_PATH = "cronosd"
DEFAULT_CHAIN_ID = "cronos_777-1"
DEFAULT_DENOM = "basecro"
# the container must be deployed with the prefixed name
HOSTNAME_TEMPLATE = "testplan-{index}"
LOCAL_RPC = "http://localhost:26657"
ECHO_SERVER_PORT = 26659


class CLI:
    def gen(
        self,
        outdir: str,
        validators: int,
        fullnodes: int,
        hostname_template=HOSTNAME_TEMPLATE,
        num_accounts=10,
        num_txs=1000,
        block_executor="block-stm",  # or "sequential"
    ):
        outdir = Path(outdir)
        cli = ChainCommand(LOCAL_CRONOSD_PATH)
        (outdir / VALIDATOR_GROUP).mkdir(parents=True, exist_ok=True)
        (outdir / FULLNODE_GROUP).mkdir(parents=True, exist_ok=True)

        peers = []
        for i in range(validators):
            print("init validator", i)
            ip = hostname_template.format(index=i)
            peers.append(init_node_local(cli, outdir, VALIDATOR_GROUP, i, ip))
        for i in range(fullnodes):
            print("init fullnode", i)
            ip = hostname_template.format(index=i + validators)
            peers.append(init_node_local(cli, outdir, FULLNODE_GROUP, i, ip))

        print("prepare genesis")
        # use a full node directory to prepare the genesis file
        genesis = gen_genesis(cli, outdir / FULLNODE_GROUP / "0", peers)

        print("patch genesis")
        # write genesis file and patch config files
        for i in range(validators):
            patch_configs_local(
                peers, genesis, outdir, VALIDATOR_GROUP, i, i, block_executor
            )
        for i in range(fullnodes):
            patch_configs_local(
                peers,
                genesis,
                outdir,
                FULLNODE_GROUP,
                i,
                i + validators,
                block_executor,
            )

        print("write config")
        cfg = {
            "validators": validators,
            "fullnodes": fullnodes,
            "num_accounts": num_accounts,
            "num_txs": num_txs,
        }
        (outdir / "config.json").write_text(json.dumps(cfg))

    def patchimage(
        self,
        toimage,
        src,
        dst="/data",
        fromimage="ghcr.io/crypto-org-chain/cronos-testground:latest",
    ):
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

    def run(
        self,
        outdir: str = "/outputs",
        datadir: str = "/data",
        cronosd=CONTAINER_CRONOSD_PATH,
        global_seq=None,
    ):
        run_echo_server(ECHO_SERVER_PORT)

        datadir = Path(datadir)
        cfg = json.loads((datadir / "config.json").read_text())

        if global_seq is None:
            global_seq = node_index()

        validators = cfg["validators"]
        group = VALIDATOR_GROUP if global_seq < validators else FULLNODE_GROUP
        group_seq = global_seq if group == VALIDATOR_GROUP else global_seq - validators
        print("node role", global_seq, group, group_seq)

        home = datadir / group / str(group_seq)

        # wait for persistent peers to be ready
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
        wait_for_w3()
        generate_load(
            cli, cfg["num_accounts"], cfg["num_txs"], home=home, output="json"
        )
        if group == VALIDATOR_GROUP:
            # validators quit when the chain is idle for a while
            detect_idle(20, 20)
        else:
            # wait more blocks to finish all tasks
            detect_idle(4, 4)

        with (home / "block_stats.log").open("w") as logfile:
            dump_block_stats(logfile)

        proc.kill()
        proc.wait(20)

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


def detect_idle(idle_blocks: int, interval: int):
    """
    returns if the chain is empty for consecutive idle_blocks
    """
    while True:
        latest = block_height()
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


def block_height():
    rsp = requests.get(f"{LOCAL_RPC}/status").json()
    return int(rsp["result"]["sync_info"]["latest_block_height"])


def block(height):
    return requests.get(f"{LOCAL_RPC}/block?height={height}").json()


def block_txs(height):
    return block(height)["result"]["block"]["data"]["txs"]


def init_node_local(
    cli: ChainCommand, outdir: Path, group: str, group_seq: int, ip: str
) -> PeerPacket:
    return init_node(
        cli,
        outdir / group / str(group_seq),
        ip,
        DEFAULT_CHAIN_ID,
        group,
        group_seq,
    )


def patch_configs_local(
    peers: List[PeerPacket],
    genesis,
    outdir: Path,
    group: str,
    i: int,
    group_seq: int,
    block_executor: str,
):
    home = outdir / group / str(i)
    (home / "config" / "genesis.json").write_text(json.dumps(genesis))
    p2p_peers = connect_all(peers[i], peers)
    patch_configs(home, group, p2p_peers, block_executor)


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


def dump_block_stats(fp):
    """
    dump simple statistics for blocks for analysis
    """
    for i in range(1, block_height() + 1):
        blk = block(i)
        timestamp = blk["result"]["block"]["header"]["time"]
        txs = len(blk["result"]["block"]["data"]["txs"])
        print("block", i, txs, timestamp, file=fp)


def main():
    fire.Fire(CLI)


if __name__ == "__main__":
    main()
