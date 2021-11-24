import json
import os
import signal
import subprocess
import time
from pathlib import Path

import tomlkit
import web3
from pystarport import ports
from web3.middleware import geth_poa_middleware

from .cosmoscli import CosmosCLI
from .utils import wait_for_port


class Cronos:
    def __init__(self, base_dir):
        self._w3 = None
        self.base_dir = base_dir
        self.config = json.load(open(base_dir / "config.json"))
        self.enable_auto_deployment = json.load(open(base_dir / "genesis.json"))[
            "app_state"
        ]["cronos"]["params"]["enable_auto_deployment"]

    @property
    def w3(self, i=0):
        if self._w3 is None:
            port = ports.evmrpc_port(self.base_port(i))
            self._w3 = web3.Web3(
                web3.providers.HTTPProvider(f"http://localhost:{port}")
            )
        return self._w3

    def base_port(self, i):
        return self.config["validators"][i]["base_port"]

    def node_rpc(self, i):
        return "tcp://127.0.0.1:%d" % ports.rpc_port(self.base_port(i))

    def cosmos_cli(self, i=0):
        return CosmosCLI(self.base_dir / f"node{i}", self.node_rpc(i), "cronosd")


class Chainmain:
    def __init__(self, base_dir):
        self.base_dir = base_dir
        self.config = json.load(open(base_dir / "config.json"))

    def base_port(self, i):
        return self.config["validators"][i]["base_port"]

    def node_rpc(self, i):
        return "tcp://127.0.0.1:%d" % ports.rpc_port(self.base_port(i))

    def cosmos_cli(self, i=0):
        return CosmosCLI(self.base_dir / f"node{i}", self.node_rpc(i), "chain-maind")


class Hermes:
    def __init__(self, base_dir):
        self.base_dir = base_dir
        configpath = base_dir / "config.toml"
        with open(configpath) as f:
            a = f.read()
            b = tomlkit.loads(a)
        self.config = b
        self.configpath = configpath

    def base_port(self, i):
        return self.config["validators"][i]["base_port"]

    def node_rpc(self, i):
        return "tcp://127.0.0.1:%d" % ports.rpc_port(self.base_port(i))

    def cosmos_cli(self, i=0):
        return CosmosCLI(self.base_dir / f"node{i}", self.node_rpc(i), "cronosd")


class Geth:
    def __init__(self, w3):
        self.w3 = w3


def setup_cronos(path, base_port, enable_auto_deployment=True):
    cfg = Path(__file__).parent / (
        "../scripts/cronos-devnet.yaml"
        if enable_auto_deployment
        else "configs/disable_auto_deployment.yaml"
    )
    yield from setup_custom_cronos(path, base_port, cfg)


def setup_chainmain(path, base_port):
    cmd = ["start-chainmain", path, "--base_port", str(base_port)]
    print(*cmd)
    proc = subprocess.Popen(
        cmd,
        preexec_fn=os.setsid,
    )
    try:
        wait_for_port(base_port)
        yield Chainmain(path / "chainmain-1")
    finally:
        os.killpg(os.getpgid(proc.pid), signal.SIGTERM)
        # proc.terminate()
        proc.wait()


def setup_hermes(path):
    cmd = ["start-hermes", path]
    proc = subprocess.Popen(
        cmd,
        preexec_fn=os.setsid,
    )
    try:
        # wait_for_port(base_port)
        time.sleep(4)
        yield Hermes(path)
    finally:
        os.killpg(os.getpgid(proc.pid), signal.SIGTERM)
        # proc.terminate()
        proc.wait()


def setup_geth(path, base_port):
    with (path / "geth.log").open("w") as logfile:
        cmd = [
            "start-geth",
            path,
            "--http.port",
            str(base_port),
            "--port",
            str(base_port + 1),
        ]
        print(*cmd)
        proc = subprocess.Popen(
            cmd,
            preexec_fn=os.setsid,
            stdout=logfile,
            stderr=subprocess.STDOUT,
        )
        try:
            wait_for_port(base_port)
            w3 = web3.Web3(web3.providers.HTTPProvider(f"http://127.0.0.1:{base_port}"))
            w3.middleware_onion.inject(geth_poa_middleware, layer=0)
            yield Geth(w3)
        finally:
            os.killpg(os.getpgid(proc.pid), signal.SIGTERM)
            # proc.terminate()
            proc.wait()


class GravityBridge:
    cronos: Cronos
    geth: Geth
    # gravity contract deployed on geth
    contract: web3.contract.Contract

    def __init__(self, cronos, geth, contract):
        self.cronos = cronos
        self.geth = geth
        self.contract = contract


def setup_custom_cronos(path, base_port, config):
    cmd = [
        "pystarport",
        "serve",
        "--config",
        config,
        "--data",
        path,
        "--base_port",
        str(base_port),
        "--quiet",
    ]
    print(*cmd)
    proc = subprocess.Popen(
        cmd,
        preexec_fn=os.setsid,
    )
    try:
        wait_for_port(ports.evmrpc_port(base_port))
        yield Cronos(path / "cronos_777-1")
    finally:
        os.killpg(os.getpgid(proc.pid), signal.SIGTERM)
        # proc.terminate()
        proc.wait()
