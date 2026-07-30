import importlib.util
import json
from pathlib import Path

from remote_benchmark.contracts import NFT_ADDRESS, POOL_ADDRESS

REPO_ROOT = Path(__file__).resolve().parents[1]


def _load_patch_module():
    spec = importlib.util.spec_from_file_location(
        "patch_erc20_genesis", REPO_ROOT / "local" / "patch_erc20_genesis.py"
    )
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


patch_module = _load_patch_module()


def _bare_genesis():
    return {
        "app_state": {
            "evm": {"accounts": []},
            "auth": {
                "accounts": [
                    {
                        "@type": "/cosmos.auth.v1beta1.BaseAccount",
                        "address": "crc1validator",
                    }
                ]
            },
        }
    }


def test_patch_genesis_adds_erc20_pool_and_nft_accounts(tmp_path):
    path = tmp_path / "genesis.json"
    path.write_text(json.dumps(_bare_genesis()))

    patch_module.patch_genesis(path, ["0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"])

    genesis = json.loads(path.read_text())
    evm_addresses = {a["address"] for a in genesis["app_state"]["evm"]["accounts"]}
    assert patch_module.CONTRACT_ADDRESS in evm_addresses
    assert POOL_ADDRESS in evm_addresses
    assert NFT_ADDRESS in evm_addresses
    # the validator account already there must survive untouched
    auth_top_level = {
        a["address"] for a in genesis["app_state"]["auth"]["accounts"] if "address" in a
    }
    assert "crc1validator" in auth_top_level


def test_patch_genesis_is_a_no_op_on_a_second_run(tmp_path):
    path = tmp_path / "genesis.json"
    path.write_text(json.dumps(_bare_genesis()))
    addresses = ["0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"]

    patch_module.patch_genesis(path, addresses)
    once = json.loads(path.read_text())["app_state"]

    patch_module.patch_genesis(path, addresses)
    twice = json.loads(path.read_text())["app_state"]

    assert once["evm"]["accounts"] == twice["evm"]["accounts"]
    assert once["auth"]["accounts"] == twice["auth"]["accounts"]
