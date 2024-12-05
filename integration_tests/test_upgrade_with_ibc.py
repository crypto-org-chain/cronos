import json
import shutil
import stat
import subprocess
from pathlib import Path

import pytest
import requests
from pystarport import cluster, ports

from .ibc_utils import (
    assert_channel_open_init,
    prepare_network,
    wait_for_check_channel_ready,
)
from .utils import do_upgrade, post_init

pytestmark = pytest.mark.upgrade


@pytest.fixture(scope="module")
def ibc(tmp_path_factory):
    path = tmp_path_factory.mktemp("upgrade")
    nix_name = "upgrade-test-package-recent"
    configdir = Path(__file__).parent
    name = "cosmovisor_with_ibc"
    cmd = [
        "nix-build",
        configdir / f"configs/{nix_name}.nix",
    ]
    print(*cmd)
    subprocess.run(cmd, check=True)

    # copy the content so the new directory is writable.
    upgrades = path / "upgrades"
    shutil.copytree("./result", upgrades)
    mod = stat.S_IRWXU
    upgrades.chmod(mod)
    for d in upgrades.iterdir():
        d.chmod(mod)

    binary = str(upgrades / "genesis/bin/cronosd")
    yield from prepare_network(
        path,
        name,
        incentivized=False,
        connection_only=True,
        post_init=post_init,
        chain_binary=f"chain-maind,{binary}",
        relayer=cluster.Relayer.RLY.value,
    )


def get_tx(base_port, hash):
    p = ports.api_port(base_port)
    url = f"http://127.0.0.1:{p}/cosmos/tx/v1beta1/txs/{hash}"
    return requests.get(url).json()


def test_cosmovisor_upgrade(ibc):
    c = ibc.cronos
    cli = c.cosmos_cli()
    connid = "connection-0"
    v = json.dumps({"fee_version": "ics29-1", "app_version": ""})
    signer = "signer2"
    rsp = cli.icaauth_register_account(
        connid,
        from_=signer,
        gas="400000",
        version=v,
    )
    ica_txhash = rsp["txhash"]
    _, channel_id = assert_channel_open_init(rsp)
    wait_for_check_channel_ready(cli, connid, channel_id)
    ica_address = cli.icaauth_query_account(
        connid,
        cli.address(signer),
    )["interchain_account_address"]
    print("ica address", ica_address, "channel_id", channel_id)
    base_port = c.base_port(0)
    ica_bf = get_tx(base_port, ica_txhash)
    cli = do_upgrade(c, "v1.4", cli.block_height() + 15)

    with pytest.raises(AssertionError):
        cli.query_params("icaauth")

    cli = do_upgrade(c, "v1.4.0-rc5-testnet", cli.block_height() + 15)
    ica_af = get_tx(base_port, ica_txhash)
    assert ica_bf == ica_af, ica_af
