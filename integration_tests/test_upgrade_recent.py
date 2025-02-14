import time

import pytest
import requests
from pystarport import ports

from .network import setup_upgrade_cronos
from .utils import (
    ADDRS,
    CONTRACTS,
    KEYS,
    deploy_contract,
    do_upgrade,
    sign_transaction,
    wait_for_new_blocks,
    wait_for_port,
)

pytestmark = pytest.mark.upgrade


@pytest.fixture(scope="module")
def custom_cronos(tmp_path_factory):
    port = 27100
    nix_name = "upgrade-test-package-recent"
    cfg_name = "cosmovisor_recent"
    yield from setup_upgrade_cronos(tmp_path_factory, port, nix_name, cfg_name)


def test_cosmovisor_upgrade(custom_cronos):
    c = custom_cronos
    do_upgrade(c, "v1.2", c.cosmos_cli().block_height() + 15)
    wait_for_port(ports.evmrpc_port(c.base_port(1)))
    w3 = c.node_w3(1)
    gas_price = w3.eth.gas_price
    erc20 = deploy_contract(
        w3,
        CONTRACTS["TestERC20A"],
        key=KEYS["validator"],
        gas_price=gas_price,
    )

    def transfer():
        tx = erc20.functions.transfer(ADDRS["community"], 10).build_transaction(
            {
                "from": ADDRS["validator"],
                "gasPrice": gas_price,
            }
        )
        signed = sign_transaction(w3, tx, KEYS["validator"])
        txhash = w3.eth.send_raw_transaction(signed.rawTransaction)
        receipt = w3.eth.wait_for_transaction_receipt(txhash)
        return receipt

    transfer()
    cli = do_upgrade(c, "v1.3", c.cosmos_cli().block_height() + 15)
    wait_for_port(ports.evmrpc_port(c.base_port(1)))
    receipt = transfer()
    print("mm-receipt", receipt)
    blk = receipt["blockNumber"]

    def trace_blk(i):
        url = f"http://127.0.0.1:{ports.evmrpc_port(c.base_port(i))}"
        params = {
            "method": "debug_traceBlockByNumber",
            "params": [hex(blk)],
            "id": 1,
            "jsonrpc": "2.0",
        }
        rsp = requests.post(url, json=params)
        assert rsp.status_code == 200
        return rsp.json()["result"]

    wait_for_new_blocks(cli, 1)
    b0 = trace_blk(0)
    b1 = trace_blk(1)
    assert b0 != b1, b0

    c.supervisorctl("stop", "cronos_777-1-node0")
    time.sleep(3)
    cli.changeset_fixdata(f"{c.base_dir}/node0/data/versiondb")
    c.supervisorctl("start", "cronos_777-1-node0")
    wait_for_port(ports.evmrpc_port(c.base_port(0)))
    b0 = trace_blk(0)
    assert b0 == b1, b0
