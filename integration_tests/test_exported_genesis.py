import json
import urllib.parse
from pathlib import Path

import pytest
import requests
from pystarport import ports

from .network import setup_custom_cronos
from .utils import ADDRS, CONTRACTS

pytestmark = pytest.mark.slow


@pytest.fixture(scope="module")
def custom_cronos(tmp_path_factory):
    path = tmp_path_factory.mktemp("cronos")
    yield from setup_custom_cronos(
        path, 26000, Path(__file__).parent / "configs/genesis_token_mapping.jsonnet"
    )


def test_exported_contract(custom_cronos):
    "demonstrate that contract state can be deployed in genesis"
    w3 = custom_cronos.w3
    abi = json.loads(CONTRACTS["TestERC20Utility"].read_text())["abi"]
    erc20 = w3.eth.contract(
        address="0x68542BD12B41F5D51D6282Ec7D91D7d0D78E4503", abi=abi
    )
    assert erc20.caller.balanceOf(ADDRS["validator"]) == 100000000000000000000000000


def test_exported_token_mapping(custom_cronos):
    cli = custom_cronos.cosmos_cli(0)
    rsp = cli.query_contract_by_denom(
        "gravity0x0000000000000000000000000000000000000000"
    )
    assert rsp["contract"] == "0x68542BD12B41F5D51D6282Ec7D91D7d0D78E4503"
    assert rsp["auto_contract"] == "0x68542BD12B41F5D51D6282Ec7D91D7d0D78E4503"
    denom = "ibc/6B5A664BF0AF4F71B2F0BAA33141E2F1321242FBD5D19762F541EC971ACB0865"
    rsp = cli.query_contract_by_denom(denom)
    expected = {
        "contract": "0x0000000000000000000000000000000000000000",
        "auto_contract": "",
    }
    assert rsp == expected
    port = ports.api_port(custom_cronos.base_port(0))
    param = urllib.parse.quote(denom, safe="")
    param = urllib.parse.quote(param, safe="")
    url = f"http://127.0.0.1:{port}/cronos/v1/contract_by_denom/{param}"
    rsp = requests.get(url).json()
    assert rsp == expected
