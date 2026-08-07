import importlib.util
import json
from pathlib import Path

from remote_benchmark.contracts import NFT_ADDRESS, POOL_ADDRESS
from remote_benchmark.utils import eth_to_bech32

REPO_ROOT = Path(__file__).resolve().parents[1]


def _load_patch_module():
    spec = importlib.util.spec_from_file_location(
        "patch_erc20_genesis",
        REPO_ROOT / "scripts" / "devnet-local" / "patch_erc20_genesis.py",
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
            "bank": {"balances": [], "supply": [{"denom": "basetcro", "amount": "0"}]},
        }
    }


def test_patch_genesis_adds_erc20_pool_and_nft_accounts(tmp_path):
    path = tmp_path / "genesis.json"
    path.write_text(json.dumps(_bare_genesis()))
    address = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

    patch_module.patch_genesis(path, [address], [address], "basetcro", 50 * 10**18)

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


def test_patch_genesis_adds_native_balances(tmp_path):
    path = tmp_path / "genesis.json"
    path.write_text(json.dumps(_bare_genesis()))
    address = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

    patch_module.patch_genesis(path, [address], [address], "basetcro", 50 * 10**18)

    balances = json.loads(path.read_text())["app_state"]["bank"]["balances"]
    entry = next(b for b in balances if b["address"] == eth_to_bech32(address))
    assert entry["coins"] == [{"denom": "basetcro", "amount": str(50 * 10**18)}]


def test_patch_genesis_creates_an_auth_account_for_every_funded_eoa(tmp_path):
    """x/bank's InitGenesis only writes the balance to the store - it never
    creates the account object the way a live SendCoins tx would, so without
    an explicit BaseAccount entry the funded EOA's first tx fails ante-handler
    signature verification with "account not found"."""
    path = tmp_path / "genesis.json"
    path.write_text(json.dumps(_bare_genesis()))
    address = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

    patch_module.patch_genesis(path, [address], [address], "basetcro", 50 * 10**18)

    auth_top_level = {
        a["address"]
        for a in json.loads(path.read_text())["app_state"]["auth"]["accounts"]
        if "address" in a
    }
    assert eth_to_bech32(address) in auth_top_level


def test_patch_genesis_bumps_supply_by_the_added_balances(tmp_path):
    path = tmp_path / "genesis.json"
    path.write_text(json.dumps(_bare_genesis()))
    addresses = [
        "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
        "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
    ]

    patch_module.patch_genesis(path, addresses, addresses, "basetcro", 50 * 10**18)

    supply = json.loads(path.read_text())["app_state"]["bank"]["supply"]
    entry = next(c for c in supply if c["denom"] == "basetcro")
    assert entry["amount"] == str(2 * 50 * 10**18)


def test_patch_genesis_appends_a_new_supply_entry_for_an_unseen_denom(tmp_path):
    path = tmp_path / "genesis.json"
    genesis = _bare_genesis()
    genesis["app_state"]["bank"]["supply"] = []
    path.write_text(json.dumps(genesis))
    address = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

    patch_module.patch_genesis(path, [address], [address], "basetcro", 50 * 10**18)

    supply = json.loads(path.read_text())["app_state"]["bank"]["supply"]
    assert supply == [{"denom": "basetcro", "amount": str(50 * 10**18)}]


def test_patch_genesis_funds_a_wider_native_range_than_erc20(tmp_path):
    """sender_strategy=unique-per-tx needs a native balance for every physical
    sender, but only the logical accounts need an ERC20 balance."""
    path = tmp_path / "genesis.json"
    path.write_text(json.dumps(_bare_genesis()))
    erc20_address = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    extra_fund_address = "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

    patch_module.patch_genesis(
        path,
        [erc20_address],
        [erc20_address, extra_fund_address],
        "basetcro",
        50 * 10**18,
    )

    genesis = json.loads(path.read_text())
    evm_addresses = {a["address"] for a in genesis["app_state"]["evm"]["accounts"]}
    bank_addresses = {b["address"] for b in genesis["app_state"]["bank"]["balances"]}
    assert extra_fund_address not in evm_addresses
    assert eth_to_bech32(extra_fund_address) in bank_addresses


def test_patch_genesis_is_a_no_op_on_a_second_run(tmp_path):
    path = tmp_path / "genesis.json"
    path.write_text(json.dumps(_bare_genesis()))
    addresses = ["0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"]

    patch_module.patch_genesis(path, addresses, addresses, "basetcro", 50 * 10**18)
    once = json.loads(path.read_text())["app_state"]

    patch_module.patch_genesis(path, addresses, addresses, "basetcro", 50 * 10**18)
    twice = json.loads(path.read_text())["app_state"]

    assert once["evm"]["accounts"] == twice["evm"]["accounts"]
    assert once["auth"]["accounts"] == twice["auth"]["accounts"]
    assert once["bank"]["balances"] == twice["bank"]["balances"]
    assert once["bank"]["supply"] == twice["bank"]["supply"]
