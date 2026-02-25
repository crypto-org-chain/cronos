import asyncio
import json
import os
import shutil
import socket
import subprocess
import sys
import tarfile
import tempfile
import threading
import time
from pathlib import Path
from typing import List, Optional

import click
import jsonmerge
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
from .stats import _fetch_prometheus, dump_block_stats, scrape_blockstm_metrics
from .topology import connect_all
from .types import PeerPacket
from .utils import (
    Tee,
    block_height,
    block_txs,
    mempool_status,
    wait_for_block,
    wait_for_port,
)

# use cronosd on host machine
LOCAL_CRONOSD_PATH = "cronosd"
DEFAULT_CHAIN_ID = "cronos_777-1"
# the container must be deployed with the prefixed name
HOSTNAME_TEMPLATE = "testplan-{index}"
ECHO_SERVER_PORT = 26659
LOCAL_RPC = "http://127.0.0.1:26657"


class MempoolMonitor:
    """Background thread that polls CometBFT mempool during the load period.

    Records the peak (n_txs, n_bytes) observed at each block height so that
    dump_block_stats can report accurate mempool pressure instead of always
    seeing 0 when queried post-hoc.
    """

    def __init__(self, rpc=LOCAL_RPC, interval=0.2):
        self._rpc = rpc
        self._interval = interval
        self._data = {}
        self._stop = threading.Event()
        self._thread = None

    def start(self):
        self._thread = threading.Thread(target=self._poll, daemon=True)
        self._thread.start()

    def stop(self):
        self._stop.set()
        if self._thread:
            self._thread.join(timeout=2)

    @property
    def data(self):
        """Dict mapping block height to (peak_n_txs, peak_n_bytes)."""
        return dict(self._data)

    def _poll(self):
        while not self._stop.is_set():
            try:
                h = block_height(self._rpc)
                n_txs, n_bytes = mempool_status(self._rpc)
                prev = self._data.get(h, (0, 0))
                self._data[h] = (max(prev[0], n_txs), max(prev[1], n_bytes))
            except Exception:
                pass
            self._stop.wait(self._interval)


class BlockSTMMonitor:
    """Background thread that records Block-STM gauges at each new block height.

    The Cosmos SDK Block-STM executor sets Prometheus gauges (not counters)
    that are overwritten on every FinalizeBlock.  This monitor polls the
    telemetry endpoint and captures the (executed_txs, validated_txs) snapshot
    whenever the block height advances, so dump_block_stats can report
    accurate per-block averages instead of a stale post-hoc value.
    """

    def __init__(self, rpc=LOCAL_RPC, interval=0.3):
        self._rpc = rpc
        self._interval = interval
        self._data = {}  # height -> (executed, validated)
        self._stop = threading.Event()
        self._thread = None
        self._last_height = 0

    def start(self):
        self._thread = threading.Thread(target=self._poll, daemon=True)
        self._thread.start()

    def stop(self):
        self._stop.set()
        if self._thread:
            self._thread.join(timeout=2)

    @property
    def data(self):
        """Dict mapping block height to (executed_txs, validated_txs)."""
        return dict(self._data)

    def _poll(self):
        while not self._stop.is_set():
            try:
                h = block_height(self._rpc)
                if h != self._last_height:
                    self._last_height = h
                    prom_text = _fetch_prometheus()
                    stm = scrape_blockstm_metrics(prom_text)
                    if stm:
                        executed = stm.get("executed_txs", 0)
                        validated = stm.get("validated_txs", 0)
                        if executed > 0:
                            self._data[h] = (executed, validated)
            except Exception:
                pass
            self._stop.wait(self._interval)


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
@click.option("--num-idle", default=20)
@click.option("--tx-type", default="simple-transfer")
@click.option("--batch-size", default=1)
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


def _resolve_node_overrides(
    defaults: dict, node_overrides: Optional[dict], global_seq: int
):
    """Deep-merge per-node overrides on top of defaults for the given node."""
    if not node_overrides:
        return defaults
    overrides = node_overrides.get(str(global_seq))
    if not overrides:
        return defaults
    return jsonmerge.merge(defaults, overrides)


def _diff_dicts(base, other, path=""):
    """Yield (dotted_path, base_val, other_val) for all leaf differences."""
    all_keys = set(list(base.keys()) + list(other.keys()))
    for k in sorted(all_keys):
        p = f"{path}.{k}" if path else k
        v1 = base.get(k)
        v2 = other.get(k)
        if isinstance(v1, dict) and isinstance(v2, dict):
            yield from _diff_dicts(v1, v2, p)
        elif v1 != v2:
            yield p, v1, v2


def _print_node_config_summary(
    validators,
    fullnodes,
    num_accounts,
    num_txs,
    tx_type,
    batch_size,
    config_patch,
    app_patch,
    node_overrides,
):
    """Print a summary table of per-node config differences when overrides exist."""
    if not node_overrides:
        return

    total = validators + fullnodes
    defaults = {
        "num_accounts": num_accounts,
        "num_txs": num_txs,
        "tx_type": tx_type,
        "batch_size": batch_size,
        "config_patch": config_patch,
        "app_patch": app_patch,
    }

    print()
    print("=" * 60)
    print("Per-node configuration differences")
    print("=" * 60)

    for seq in range(total):
        overrides = node_overrides.get(str(seq))
        if not overrides:
            continue
        role = "validator" if seq < validators else "fullnode"
        group_seq = seq if seq < validators else seq - validators
        resolved = _resolve_node_overrides(defaults, node_overrides, seq)
        diffs = list(_diff_dicts(defaults, resolved))
        if not diffs:
            continue
        print(f"\n  {role} {group_seq} (global_seq={seq}):")
        for path, old, new in diffs:
            print(f"    {path}: {old} -> {new}")

    print()
    print("=" * 60)
    print()


def _gen(
    outdir: str,
    hostname_template: str = HOSTNAME_TEMPLATE,
    validators: int = 3,
    fullnodes: int = 7,
    num_accounts: int = 10,
    num_txs: int = 1000,
    num_idle: int = 20,
    tx_type: str = "simple-transfer",
    batch_size: int = 1,
    validator_generate_load: bool = True,
    config_patch: dict = None,
    app_patch: dict = None,
    genesis_patch: dict = None,
    node_overrides: Optional[dict] = None,
    **_kwargs,
):
    config_patch = config_patch or {}
    app_patch = app_patch or {}
    genesis_patch = genesis_patch or {}
    node_overrides = node_overrides or {}

    outdir = Path(outdir)
    cli = ChainCommand(LOCAL_CRONOSD_PATH)
    (outdir / VALIDATOR_GROUP).mkdir(parents=True, exist_ok=True)
    (outdir / FULLNODE_GROUP).mkdir(parents=True, exist_ok=True)

    peers = []
    for i in range(validators):
        print("init validator", i)
        global_seq = i
        ip = hostname_template.format(index=global_seq)
        node_cfg = _resolve_node_overrides(
            {"num_accounts": num_accounts}, node_overrides, global_seq
        )
        peers.append(
            init_node_local(
                cli,
                outdir,
                VALIDATOR_GROUP,
                i,
                global_seq,
                ip,
                node_cfg["num_accounts"],
            )
        )
    for i in range(fullnodes):
        print("init fullnode", i)
        global_seq = i + validators
        ip = hostname_template.format(index=global_seq)
        node_cfg = _resolve_node_overrides(
            {"num_accounts": num_accounts}, node_overrides, global_seq
        )
        peers.append(
            init_node_local(
                cli,
                outdir,
                FULLNODE_GROUP,
                i,
                global_seq,
                ip,
                node_cfg["num_accounts"],
            )
        )

    print("prepare genesis")
    # use a full node directory to prepare the genesis file
    genesis = gen_genesis(cli, outdir / VALIDATOR_GROUP / "0", peers, genesis_patch)

    print("patch genesis")
    # write genesis file and patch config files, applying per-node overrides
    for i in range(validators):
        node_cfg = _resolve_node_overrides(
            {"config_patch": config_patch, "app_patch": app_patch},
            node_overrides,
            i,
        )
        patch_configs_local(
            peers,
            genesis,
            outdir,
            VALIDATOR_GROUP,
            i,
            node_cfg["config_patch"],
            node_cfg["app_patch"],
        )
    for i in range(fullnodes):
        global_seq = i + validators
        node_cfg = _resolve_node_overrides(
            {"config_patch": config_patch, "app_patch": app_patch},
            node_overrides,
            global_seq,
        )
        patch_configs_local(
            peers,
            genesis,
            outdir,
            FULLNODE_GROUP,
            i,
            node_cfg["config_patch"],
            node_cfg["app_patch"],
        )

    print("write config")
    cfg = {
        "validators": validators,
        "fullnodes": fullnodes,
        "num_accounts": num_accounts,
        "num_txs": num_txs,
        "num_idle": num_idle,
        "tx_type": tx_type,
        "batch_size": batch_size,
        "validator_generate_load": validator_generate_load,
    }
    if node_overrides:
        cfg["node_overrides"] = node_overrides
    (outdir / "config.json").write_text(json.dumps(cfg))

    _print_node_config_summary(
        validators,
        fullnodes,
        num_accounts,
        num_txs,
        tx_type,
        batch_size,
        config_patch,
        app_patch,
        node_overrides,
    )


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
@click.option("--global-seq", default=None, type=int)
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
        try:
            with tarfile.open(output, "x:bz2") as tar:
                tar.add(home, arcname="data", filter=output_filter(group, group_seq))
        except OSError:
            # ignore if the file is not writable when running in bare metal
            pass
        else:
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
@click.option("--tx-type", default="simple-transfer")
@click.option("--batch-size", default=1)
@click.option("--node", type=int)
def gen_txs(**kwargs):
    return _gen_txs(**kwargs)


@cli.command()
@click.argument("options", callback=validate_json)
def generic_gen_txs(options: dict):
    return _gen_txs(**options)


@cli.command()
@click.option("--datadir", default="/data", type=Path)
@click.option("--global-seq", default=0)
def generate_load(datadir: Path, global_seq: int):
    """
    manually generate load to an existing node
    """
    cfg = json.loads((datadir / "config.json").read_text())
    node_cfg = _resolve_node_overrides(cfg, cfg.get("node_overrides"), global_seq)
    txs = prepare_txs(node_cfg, datadir, global_seq)
    asyncio.run(transaction.send(txs))
    print("sent", len(txs), "txs")
    print("wait for 20 idle blocks")
    detect_idle_halted(node_cfg["num_idle"], 20)
    dump_block_stats(sys.stdout)


def _gen_txs(
    outdir: str,
    nodes: int = 10,
    num_accounts: int = 10,
    num_txs: int = 1000,
    tx_type: str = "simple-transfer",
    batch_size: int = 1,
    node: Optional[int] = None,
    node_overrides: Optional[dict] = None,
):
    outdir = Path(outdir)
    node_overrides = node_overrides or {}
    defaults = {
        "num_accounts": num_accounts,
        "num_txs": num_txs,
        "tx_type": tx_type,
        "batch_size": batch_size,
    }

    def job(global_seq):
        cfg = _resolve_node_overrides(defaults, node_overrides, global_seq)
        na, nt = cfg["num_accounts"], cfg["num_txs"]
        print("generating", na * nt, "txs for node", global_seq)
        txs = transaction.gen(global_seq, na, nt, cfg["tx_type"], cfg["batch_size"])
        transaction.save(txs, outdir, global_seq)
        print("saved", len(txs), "txs for node", global_seq)

    if node is not None:
        job(node)
    else:
        for global_seq in range(nodes):
            job(global_seq)


def do_run(
    datadir: Path, home: Path, cronosd: str, group: str, global_seq: int, cfg: dict
):
    node_cfg = _resolve_node_overrides(cfg, cfg.get("node_overrides"), global_seq)

    if cfg.get("node_overrides", {}).get(str(global_seq)):
        diffs = list(_diff_dicts(cfg, node_cfg))
        if diffs:
            print(f"[node {global_seq}] config overrides applied:")
            for path, old, new in diffs:
                if path == "node_overrides" or path.startswith("node_overrides."):
                    continue
                print(f"  {path}: {old} -> {new}")

    if group == FULLNODE_GROUP or node_cfg.get("validator_generate_load", True):
        txs = prepare_txs(node_cfg, datadir, global_seq)
    else:
        txs = []

    # wait for persistent peers to be ready
    run_echo_server(ECHO_SERVER_PORT)
    wait_for_peers(home)

    print("start node")
    logfile = open(home / "node.log", "ab", buffering=0)
    proc = subprocess.Popen(
        [cronosd, "start", "--home", str(home), "--async-check-tx"],
        stdout=logfile,
    )

    cli = ChainCommand(cronosd)
    wait_for_port(26657)
    wait_for_port(8545)
    wait_for_block(cli, 3)

    monitor = MempoolMonitor()
    monitor.start()
    stm_monitor = BlockSTMMonitor()
    stm_monitor.start()

    if txs:
        # Send in a background thread so blocks are produced concurrently.
        # Pacing with time.sleep prevents the CheckTx flood that saturates
        # the CPU and starves the consensus goroutine, causing repeated
        # round timeouts at the Propose step.
        chunk_size = node_cfg.get("send_batch_size", 2000)
        send_interval = node_cfg.get("send_interval", 0.2)

        def _send_paced():
            for i in range(0, len(txs), chunk_size):
                chunk = txs[i : i + chunk_size]
                asyncio.run(transaction.send(chunk, batch_size=chunk_size))
                if i + chunk_size < len(txs):
                    time.sleep(send_interval)
            print("sent", len(txs), "txs")

        sender = threading.Thread(target=_send_paced, daemon=True)
        sender.start()

    # node quit when the chain is idle or halted for a while
    detect_idle_halted(node_cfg["num_idle"], 5)

    if txs:
        sender.join(timeout=30)

    monitor.stop()
    stm_monitor.stop()

    with (home / "block_stats.log").open("w") as logfile:
        dump_block_stats(
            Tee(logfile, sys.stdout),
            mempool_data=monitor.data,
            stm_data=stm_monitor.data,
        )

    if cfg.get("node_overrides", {}).get(str(global_seq)):
        print()
        print(f"[node {global_seq}] effective config vs defaults:")
        for path, old, new in _diff_dicts(cfg, node_cfg):
            if path == "node_overrides" or path.startswith("node_overrides."):
                continue
            print(f"  {path}: {new}  (default: {old})")

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
        parts = peer.split("@", 1)
        if len(parts) < 2:
            # ignore invalid or empty peer
            continue
        host = parts[1].split(":", 1)[0]
        print("wait for peer to be ready:", host)
        wait_for_port(ECHO_SERVER_PORT, host=host, timeout=2400)


def prepare_txs(cfg, datadir, global_seq):
    txs = transaction.load(datadir, global_seq)
    if txs:
        print("loaded", len(txs), "txs")
    else:
        print("generating", cfg["num_accounts"] * cfg["num_txs"], "txs")
        txs = transaction.gen(
            global_seq,
            cfg["num_accounts"],
            cfg["num_txs"],
            cfg["tx_type"],
            cfg["batch_size"],
        )
    return txs


if __name__ == "__main__":
    cli()
