import os
import signal
import subprocess

import pytest
from pystarport import cluster

from .ibc_utils import (
    RATIO,
    cronos_transfer_source_tokens,
    cronos_transfer_source_tokens_with_proxy,
    get_balance,
    ibc_incentivized_transfer,
    prepare_network,
)
from .utils import ADDRS, eth_to_bech32, wait_for_fn, wait_for_new_blocks

cronos_signer2 = ADDRS["signer2"]
src_amount = 10
src_denom = "basecro"
dst_amount = src_amount * RATIO  # the decimal places difference
dst_denom = "basetcro"
channel = "channel-0"


@pytest.fixture(scope="module")
def ibc(request, tmp_path_factory):
    "prepare-network"
    path = tmp_path_factory.mktemp("ibc_rly")
    procs = []
    try:
        for network in prepare_network(
            path,
            "ibc",
            True,
            True,
            cluster.Relayer.RLY.value,
        ):
            if network.proc:
                procs.append(network.proc)
                print("append:", network.proc)
            yield network
    finally:
        print("finally:", procs)
        for proc in procs:
            try:
                os.killpg(os.getpgid(proc.pid), signal.SIGTERM)
            except ProcessLookupError:
                pass
            # proc.terminate()
            proc.wait()


def rly_transfer(ibc):
    # chainmain-1 -> cronos_777-1
    my_ibc0 = "chainmain-1"
    my_ibc1 = "cronos_777-1"
    path = ibc.cronos.base_dir.parent / "relayer"
    # srcchainid dstchainid amount dst_addr srchannelid
    cmd = (
        f"rly tx transfer {my_ibc0} {my_ibc1} {src_amount}{src_denom} "
        f"{eth_to_bech32(cronos_signer2)} {channel} "
        f"--path chainmain-cronos "
        f"--home {str(path)}"
    )
    subprocess.run(cmd, check=True, shell=True)


def test_ibc(ibc):
    # chainmain-1 relayer -> cronos_777-1 signer2
    wait_for_new_blocks(ibc.cronos.cosmos_cli(), 1)
    rly_transfer(ibc)
    dst_addr = eth_to_bech32(cronos_signer2)
    old_dst_balance = get_balance(ibc.cronos, dst_addr, dst_denom)
    new_dst_balance = 0

    def check_balance_change():
        nonlocal new_dst_balance
        new_dst_balance = get_balance(ibc.cronos, dst_addr, dst_denom)
        return new_dst_balance != old_dst_balance

    wait_for_fn("balance change", check_balance_change)
    assert old_dst_balance + dst_amount == new_dst_balance


def test_ibc_incentivized_transfer(ibc):
    cli = ibc.cronos.cosmos_cli()
    wait_for_new_blocks(cli, 1)
    ibc_incentivized_transfer(ibc)


def test_cronos_transfer_source_tokens(ibc):
    cli = ibc.cronos.cosmos_cli()
    wait_for_new_blocks(cli, 1)
    cronos_transfer_source_tokens(ibc)


def test_cronos_transfer_source_tokens_with_proxy(ibc):
    cli = ibc.cronos.cosmos_cli()
    wait_for_new_blocks(cli, 1)
    cronos_transfer_source_tokens_with_proxy(ibc)
