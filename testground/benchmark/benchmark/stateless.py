import json
import os
import socket
import subprocess
from pathlib import Path
from typing import List

import fire

from .cli import ChainCommand
from .peer import (
    CONTAINER_CRONOSD_PATH,
    FULLNODE_GROUP,
    VALIDATOR_GROUP,
    gen_genesis,
    init_node,
    patch_configs,
)
from .topology import connect_all
from .types import PeerPacket
from .utils import wait_for_block, wait_for_port

# use cronosd on host machine
LOCAL_CRONOSD_PATH = "cronosd"
DEFAULT_CHAIN_ID = "cronos_777-1"
DEFAULT_DENOM = "basecro"
# the container must be deployed with the prefixed name
CONTAINER_PREFIX = "testplan-"


class CLI:
    def gen(self, outdir: str, validators: int, fullnodes: int):
        outdir = Path(outdir)
        cli = ChainCommand(LOCAL_CRONOSD_PATH)
        (outdir / VALIDATOR_GROUP).mkdir(parents=True, exist_ok=True)
        (outdir / FULLNODE_GROUP).mkdir(parents=True, exist_ok=True)

        peers = []
        for i in range(validators):
            print("init validator", i)
            peers.append(init_node_local(cli, outdir, VALIDATOR_GROUP, i, i))
        for i in range(fullnodes):
            print("init fullnode", i)
            peers.append(
                init_node_local(cli, outdir, FULLNODE_GROUP, i, i + validators)
            )

        print("prepare genesis")
        # use a full node directory to prepare the genesis file
        genesis = gen_genesis(cli, outdir / FULLNODE_GROUP / "0", peers)

        print("patch genesis")
        # write genesis file and patch config files
        for i in range(validators):
            patch_configs_local(peers, genesis, outdir, VALIDATOR_GROUP, i, i)
        for i in range(fullnodes):
            patch_configs_local(
                peers, genesis, outdir, FULLNODE_GROUP, i, i + validators
            )

    def run(
        self,
        outdir: str,
        validators: int,
        cronosd=CONTAINER_CRONOSD_PATH,
        global_seq=None,
    ):
        outdir = Path(outdir)
        if global_seq is None:
            global_seq = node_index()
        group = VALIDATOR_GROUP if global_seq < validators else FULLNODE_GROUP
        group_seq = global_seq if group == VALIDATOR_GROUP else global_seq - validators
        print("node role", global_seq, group, group_seq)

        home = outdir / group / str(group_seq)

        # start the node
        logfile = home / "node.log"
        proc = subprocess.Popen(
            [cronosd, "start", "--home", str(home)],
            stdout=open(logfile, "ab", buffering=0),
        )

        cli = ChainCommand(cronosd)
        wait_for_port(26657)
        wait_for_port(8545)
        wait_for_block(cli, 1)

        proc.kill()
        try:
            proc.wait(5)
        except subprocess.TimeoutExpired:
            pass


def init_node_local(
    cli: ChainCommand, outdir: Path, group: str, group_seq: int, global_seq: int
) -> PeerPacket:
    return init_node(
        cli,
        outdir / group / str(group_seq),
        CONTAINER_PREFIX + str(global_seq),
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
):
    home = outdir / group / str(i)
    (home / "config" / "genesis.json").write_text(json.dumps(genesis))
    p2p_peers = connect_all(peers[i], peers)
    patch_configs(home, group, p2p_peers)


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


def main():
    fire.Fire(CLI)


if __name__ == "__main__":
    main()
