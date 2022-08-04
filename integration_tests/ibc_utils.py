import json
import subprocess
from pathlib import Path
from typing import NamedTuple

from pystarport import ports

from .network import Chainmain, Cronos, Hermes, setup_custom_cronos
from .utils import ADDRS, eth_to_bech32, supervisorctl, wait_for_port

RATIO = 10**10


class IBCNetwork(NamedTuple):
    cronos: Cronos
    chainmain: Chainmain
    hermes: Hermes


def prepare_network(tmp_path, file):
    file = f"configs/{file}.jsonnet"
    gen = setup_custom_cronos(tmp_path, 26700, Path(__file__).parent / file)
    cronos = next(gen)
    chainmain = Chainmain(cronos.base_dir.parent / "chainmain-1")
    hermes = Hermes(cronos.base_dir.parent / "relayer.toml")
    # wait for grpc ready
    wait_for_port(ports.grpc_port(chainmain.base_port(0)))  # chainmain grpc
    wait_for_port(ports.grpc_port(cronos.base_port(0)))  # cronos grpc
    subprocess.check_call(
        [
            "hermes",
            "-c",
            hermes.configpath,
            "create",
            "channel",
            "cronos_777-1",
            "chainmain-1",
            "--port-a",
            "transfer",
            "--port-b",
            "transfer",
        ]
    )
    supervisorctl(cronos.base_dir / "../tasks.ini", "start", "relayer-demo")
    wait_for_port(hermes.port)
    yield IBCNetwork(cronos, chainmain, hermes)


def assert_ready(ibc):
    # wait for hermes
    output = subprocess.getoutput(
        f"curl -s -X GET 'http://127.0.0.1:{ibc.hermes.port}/state' | jq"
    )
    assert json.loads(output)["status"] == "success"


def prepare(ibc):
    assert_ready(ibc)
    # chainmain-1 -> cronos_777-1
    my_ibc0 = "chainmain-1"
    my_ibc1 = "cronos_777-1"
    my_channel = "channel-0"
    dst_addr = eth_to_bech32(ADDRS["signer2"])
    src_amount = 10
    src_denom = "basecro"
    # dstchainid srcchainid srcportid srchannelid
    cmd = (
        f"hermes -c {ibc.hermes.configpath} tx raw ft-transfer "
        f"{my_ibc1} {my_ibc0} transfer {my_channel} {src_amount} "
        f"-o 1000 -n 1 -d {src_denom} -r {dst_addr} -k relayer"
    )
    subprocess.run(cmd, check=True, shell=True)
    return src_amount


def get_balance(chain, addr, denom):
    balance = chain.cosmos_cli().balance(addr, denom)
    print("balance", balance, addr, denom)
    return balance
