import json

import pytest
from pystarport import ports

from .utils import wait_for_port


def test_config_client_id(cronos):
    n0 = "cronos_777-1-node0"
    p0 = cronos.base_port(0)
    w3 = cronos.w3
    cronos.supervisorctl("stop", n0)
    cli = cronos.cosmos_cli(0)
    dir = cli.data_dir / "config"

    def edit_gensis_cfg(chain_id):
        genesis_cfg = dir / "genesis.json"
        genesis = json.loads(genesis_cfg.read_text())
        genesis["chain_id"] = chain_id
        genesis_cfg.write_text(json.dumps(genesis))

    # start with empty chain_id from genesis should fail
    edit_gensis_cfg("")
    with pytest.raises(Exception):
        cronos.supervisorctl("start", n0)

    edit_gensis_cfg("cronos_777-1")
    cronos.supervisorctl("start", n0)
    wait_for_port(ports.evmrpc_port(p0))
    assert w3.eth.chain_id == 777
