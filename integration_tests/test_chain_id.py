import json
from pathlib import Path

import pytest
from pystarport import ports

from .network import setup_custom_cronos
from .utils import wait_for_block, wait_for_port


@pytest.fixture(scope="module")
def custom_cronos(tmp_path_factory):
    path = tmp_path_factory.mktemp("cronos")
    yield from setup_custom_cronos(
        path, 27100, Path(__file__).parent / "configs/default.jsonnet"
    )


def test_config_client_id(custom_cronos):
    n0 = "cronos_777-1-node0"
    p0 = custom_cronos.base_port(0)
    w3 = custom_cronos.w3
    custom_cronos.supervisorctl("stop", n0)
    cli = custom_cronos.cosmos_cli(0)
    dir = cli.data_dir / "config"

    def assert_chain_id(chain_id, timeout=None):
        genesis_cfg = dir / "genesis.json"
        genesis = json.loads(genesis_cfg.read_text())
        genesis["chain_id"] = f"cronos_{chain_id}-1"
        genesis_cfg.write_text(json.dumps(genesis))
        custom_cronos.supervisorctl("start", n0)
        wait_for_port(ports.evmrpc_port(p0))
        assert w3.eth.chain_id == chain_id
        height = w3.eth.get_block_number() + 2
        # check CONSENSUS FAILURE
        if timeout is None:
            wait_for_block(cli, height)
        else:
            with pytest.raises(TimeoutError):
                wait_for_block(cli, height, timeout)

    assert_chain_id(776, 5)
    custom_cronos.supervisorctl("stop", n0)
    assert_chain_id(777)
