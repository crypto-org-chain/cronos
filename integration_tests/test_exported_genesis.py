import json
from pathlib import Path

import pytest

from .network import setup_custom_cronos
from .utils import ADDRS


@pytest.fixture(scope="module")
def custom_cronos(tmp_path_factory):
    path = tmp_path_factory.mktemp("cronos")
    yield from setup_custom_cronos(
        path, 26000, Path(__file__).parent / "configs/genesis_token_mapping.yaml"
    )


def test_exported_contract(custom_cronos):
    "demonstrate that contract state can be deployed in genesis"
    w3 = custom_cronos.w3
    abi = json.load(
        (
            Path(__file__).parent
            / "contracts/artifacts/contracts/TestERC20Utility.sol/TestERC20Utility.json"
        ).open()
    )["abi"]
    erc20 = w3.eth.contract(
        address="0x68542BD12B41F5D51D6282Ec7D91D7d0D78E4503", abi=abi
    )
    assert erc20.caller.balanceOf(ADDRS["validator"]) == 100000000000000000000000000
