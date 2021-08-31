import json
import os
import signal
import subprocess

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


class Geth:
    def __init__(self, w3):
        self.w3 = w3


def setup_cronos(path, base_port):
    cmd = ["start-cronos", path, "--base_port", str(base_port)]
    print(*cmd)
    proc = subprocess.Popen(
        cmd,
        preexec_fn=os.setsid,
    )
    try:
        wait_for_port(ports.evmrpc_port(ports.evmrpc_port(base_port)))
        yield Cronos(path / "cronos_777-1")
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
    # web3 client of geth
    geth: web3.Web3
    # gravity contract
    contract: web3.contract.Contract

    def __init__(self, cronos, geth, contract):
        self.cronos = cronos
        self.geth = geth
        self.contract = contract
