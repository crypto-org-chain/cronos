import json
from pathlib import Path

import pytest
from pystarport import ports

from .network import setup_custom_cronos
from .utils import wait_for_port


@pytest.fixture(scope="module")
def custom_cronos(tmp_path_factory):
    path = tmp_path_factory.mktemp("cronos")
    yield from setup_custom_cronos(
        path,
        26800,
        Path(__file__).parent / "configs/default.jsonnet",
    )


def test_config_client_id(custom_cronos):
    n0 = "cronos_777-1-node0"
    p0 = custom_cronos.base_port(0)
    w3 = custom_cronos.w3
    custom_cronos.supervisorctl("stop", n0)
    cli = custom_cronos.cosmos_cli(0)
    dir = cli.data_dir / "config"

    def edit_gensis_cfg(chain_id):
        genesis_cfg = dir / "genesis.json"
        genesis = json.loads(genesis_cfg.read_text())
        genesis["chain_id"] = chain_id
        genesis_cfg.write_text(json.dumps(genesis))

    # start with empty chain_id from genesis should fail
    edit_gensis_cfg("")
    with pytest.raises(Exception):
        custom_cronos.supervisorctl("start", n0)

    edit_gensis_cfg("cronos_777-1")
    custom_cronos.supervisorctl("start", n0)
    wait_for_port(ports.evmrpc_port(p0))
    assert w3.eth.chain_id == 777
