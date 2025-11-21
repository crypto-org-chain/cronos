import json
import os
import signal
import subprocess
from pathlib import Path

import tomlkit
import web3
from pystarport import cluster, ports
from web3.middleware import ExtraDataToPOAMiddleware

from .cosmoscli import CosmosCLI
from .utils import supervisorctl, w3_wait_for_block, wait_for_port


class Cronos:
    def __init__(self, base_dir, chain_binary="cronosd"):
        self._w3 = None
        self.base_dir = base_dir
        self.config = json.loads((base_dir / "config.json").read_text())
        self.enable_auto_deployment = json.loads(
            (base_dir / "genesis.json").read_text()
        )["app_state"]["cronos"]["params"]["enable_auto_deployment"]
        self._use_websockets = False
        self.chain_binary = chain_binary

    def copy(self):
        return Cronos(self.base_dir)

    def w3_http_endpoint(self, i=0):
        port = ports.evmrpc_port(self.base_port(i))
        return f"http://localhost:{port}"

    def w3_ws_endpoint(self, i=0):
        port = ports.evmrpc_ws_port(self.base_port(i))
        return f"ws://localhost:{port}"

    @property
    def w3(self):
        if self._w3 is None:
            self._w3 = self.node_w3(0)
        return self._w3

    def node_w3(self, i=0):
        if self._use_websockets:
            return web3.Web3(
                web3.providers.LegacyWebSocketProvider(self.w3_ws_endpoint(i))
            )
        else:
            return web3.Web3(web3.providers.HTTPProvider(self.w3_http_endpoint(i)))

    def base_port(self, i):
        return self.config["validators"][i]["base_port"]

    def node_rpc(self, i):
        return "tcp://127.0.0.1:%d" % ports.rpc_port(self.base_port(i))

    def cosmos_cli(self, i=0) -> CosmosCLI:
        return CosmosCLI(self.node_home(i), self.node_rpc(i), self.chain_binary)

    def node_home(self, i=0):
        return self.base_dir / f"node{i}"

    def use_websocket(self, use=True):
        self._w3 = None
        self._use_websockets = use

    def supervisorctl(self, *args):
        return supervisorctl(self.base_dir / "../tasks.ini", *args)


class Chainmain:
    def __init__(self, base_dir):
        self.base_dir = base_dir
        self.config = json.loads((base_dir / "config.json").read_text())

    def base_port(self, i):
        return self.config["validators"][i]["base_port"]

    def node_rpc(self, i):
        return "tcp://127.0.0.1:%d" % ports.rpc_port(self.base_port(i))

    def cosmos_cli(self, i=0):
        return CosmosCLI(self.base_dir / f"node{i}", self.node_rpc(i), "chain-maind")


class Hermes:
    def __init__(self, config: Path):
        self.configpath = config
        self.config = tomlkit.loads(config.read_text())
        self.port = 3000


class Geth:
    def __init__(self, w3):
        self.w3 = w3

class AttestationLayer:
    """Wrapper for Attestation Layer chain"""

    def __init__(self, base_dir):
        self.base_dir = base_dir
        self.config = json.loads((base_dir / "config.json").read_text())

    def base_port(self, i):
        return self.config["validators"][i]["base_port"]

    def node_rpc(self, i):
        from pystarport import ports

        return f"tcp://127.0.0.1:{ports.rpc_port(self.base_port(i))}"

    def cosmos_cli(self, i=0):
        from .cosmoscli import CosmosCLI

        return CosmosCLI(
            self.base_dir / f"node{i}", self.node_rpc(i), "cronos-attestad"
        )

def setup_cronos(path, base_port, enable_auto_deployment=True):
    cfg = Path(__file__).parent / (
        "configs/default.jsonnet"
        if enable_auto_deployment
        else "configs/disable_auto_deployment.jsonnet"
    )
    yield from setup_custom_cronos(path, base_port, cfg)


def setup_geth(path, base_port):
    with (path / "geth.log").open("w") as logfile:
        cmd = [
            "start-geth",
            path,
            "--http.port",
            str(base_port),
            "--port",
            str(base_port + 1),
            "--networkid",
            str(15),
            "--miner.etherbase",
            "0x57f96e6B86CdeFdB3d412547816a82E3E0EbF9D2",
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
            w3.middleware_onion.inject(ExtraDataToPOAMiddleware, layer=0)
            yield Geth(w3)
        finally:
            try:
                os.killpg(os.getpgid(proc.pid), signal.SIGTERM)
            except ProcessLookupError:
                pass
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


def setup_custom_cronos(
    path,
    base_port,
    config,
    post_init=None,
    chain_binary=None,
    wait_port=True,
    relayer=cluster.Relayer.HERMES.value,
):
    cmd = [
        "pystarport",
        "init",
        "--config",
        config,
        "--data",
        path,
        "--base_port",
        str(base_port),
        "--no_remove",
    ]
    if relayer == cluster.Relayer.RLY.value:
        cmd = cmd + ["--relayer", str(relayer)]
    if chain_binary is not None:
        cmd = cmd[:1] + ["--cmd", chain_binary] + cmd[1:]
    print(*cmd)
    subprocess.run(cmd, check=True)
    if post_init is not None:
        post_init(path, base_port, config)
    proc = subprocess.Popen(
        ["pystarport", "start", "--data", path, "--quiet"],
        preexec_fn=os.setsid,
    )
    try:
        if wait_port:
            wait_for_port(ports.evmrpc_port(base_port))
            wait_for_port(ports.evmrpc_ws_port(base_port))
        c = Cronos(path / "cronos_777-1", chain_binary=chain_binary or "cronosd")
        w3_wait_for_block(c.w3, 1)
        yield c
    finally:
        os.killpg(os.getpgid(proc.pid), signal.SIGTERM)
        # proc.terminate()
        proc.wait()
