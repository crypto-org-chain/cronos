import json
import subprocess
from datetime import datetime, timedelta
from pathlib import Path

import pytest
from pystarport import ports
from pystarport.cluster import SUPERVISOR_CONFIG_FILE

from .network import Cronos, setup_custom_cronos
from .utils import (
    ADDRS,
    CONTRACTS,
    approve_proposal,
    deploy_contract,
    edit_ini_sections,
    send_transaction,
    wait_for_block,
    wait_for_new_blocks,
    wait_for_port,
)


def init_cosmovisor(home):
    """
    build and setup cosmovisor directory structure in each node's home directory
    """
    cosmovisor = home / "cosmovisor"
    cosmovisor.mkdir()
    (cosmovisor / "upgrades").symlink_to("../../../upgrades")
    (cosmovisor / "genesis").symlink_to("./upgrades/genesis")


def post_init(path, base_port, config):
    """
    prepare cosmovisor for each node
    """
    chain_id = "cronos_777-1"
    data = path / chain_id
    cfg = json.loads((data / "config.json").read_text())
    for i, _ in enumerate(cfg["validators"]):
        home = data / f"node{i}"
        init_cosmovisor(home)

    edit_ini_sections(
        chain_id,
        data / SUPERVISOR_CONFIG_FILE,
        lambda i, _: {
            "command": f"cosmovisor start --home %(here)s/node{i}",
            "environment": f"DAEMON_NAME=cronosd,DAEMON_HOME=%(here)s/node{i}",
        },
    )


@pytest.fixture(scope="module")
def custom_cronos(tmp_path_factory):
    path = tmp_path_factory.mktemp("upgrade")
    cmd = [
        "nix-build",
        Path(__file__).parent / "configs/upgrade-test-package.nix",
        "-o",
        path / "upgrades",
    ]
    print(*cmd)
    subprocess.run(cmd, check=True)
    # init with genesis binary
    yield from setup_custom_cronos(
        path,
        26100,
        Path(__file__).parent / "configs/cosmovisor.jsonnet",
        post_init=post_init,
        chain_binary=str(path / "upgrades/genesis/bin/cronosd"),
    )


def test_cosmovisor_upgrade(custom_cronos: Cronos, tmp_path_factory):
    """
    - propose an upgrade and pass it
    - wait for it to happen
    - it should work transparently
    """
    cli = custom_cronos.cosmos_cli()
    # export genesis from cronos v0.8.x
    custom_cronos.supervisorctl("stop", "all")
    migrate = tmp_path_factory.mktemp("migrate")
    file_path0 = Path(migrate / "v0.8.json")
    with open(file_path0, "w") as fp:
        json.dump(json.loads(cli.export()), fp)
        fp.flush()

    custom_cronos.supervisorctl("start", "cronos_777-1-node0", "cronos_777-1-node1")
    wait_for_port(ports.evmrpc_port(custom_cronos.base_port(0)))
    wait_for_new_blocks(cli, 1)

    height = cli.block_height()
    target_height = height + 15
    print("upgrade height", target_height)

    w3 = custom_cronos.w3
    contract = deploy_contract(w3, CONTRACTS["TestERC20A"])
    old_height = w3.eth.block_number
    old_balance = w3.eth.get_balance(ADDRS["validator"], block_identifier=old_height)
    old_base_fee = w3.eth.get_block(old_height).baseFeePerGas
    old_erc20_balance = contract.caller(block_identifier=old_height).balanceOf(
        ADDRS["validator"]
    )
    print("old values", old_height, old_balance, old_base_fee)

    plan_name = "v2.0.0-testnet3"
    rsp = cli.gov_propose_legacy(
        "community",
        "software-upgrade",
        {
            "name": plan_name,
            "title": "upgrade test",
            "description": "ditto",
            "upgrade-height": target_height,
            "deposit": "10000basetcro",
        },
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    approve_proposal(custom_cronos, rsp)

    # update cli chain binary
    custom_cronos.chain_binary = (
        Path(custom_cronos.chain_binary).parent.parent.parent
        / f"{plan_name}/bin/cronosd"
    )
    cli = custom_cronos.cosmos_cli()

    # block should pass the target height
    wait_for_block(cli, target_height + 2, timeout=480)
    wait_for_port(ports.rpc_port(custom_cronos.base_port(0)))

    # test migrate keystore
    cli.migrate_keystore()

    # check basic tx works
    wait_for_port(ports.evmrpc_port(custom_cronos.base_port(0)))
    receipt = send_transaction(
        custom_cronos.w3,
        {
            "to": ADDRS["community"],
            "value": 1000,
            "maxFeePerGas": 1000000000000,
            "maxPriorityFeePerGas": 10000,
        },
    )
    assert receipt.status == 1

    # query json-rpc on older blocks should success
    assert old_balance == w3.eth.get_balance(
        ADDRS["validator"], block_identifier=old_height
    )
    assert old_base_fee == w3.eth.get_block(old_height).baseFeePerGas

    # check eth_call works on older blocks
    assert old_erc20_balance == contract.caller(block_identifier=old_height).balanceOf(
        ADDRS["validator"]
    )

    # check gravity params
    assert cli.query_gravity_params() == {
        "params": {
            "gravity_id": "cronos_gravity_testnet",
            "contract_source_hash": "",
            "bridge_ethereum_address": "0x0000000000000000000000000000000000000000",
            "bridge_chain_id": "0",
            "signed_signer_set_txs_window": "10000",
            "signed_batches_window": "10000",
            "ethereum_signatures_window": "10000",
            "target_eth_tx_timeout": "43200000",
            "average_block_time": "5000",
            "average_ethereum_block_time": "15000",
            "slash_fraction_signer_set_tx": "0.001000000000000000",
            "slash_fraction_batch": "0.001000000000000000",
            "slash_fraction_ethereum_signature": "0.001000000000000000",
            "slash_fraction_conflicting_ethereum_signature": "0.001000000000000000",
            "unbond_slashing_signer_set_txs_window": "10000",
            "bridge_active": False,
            "batch_creation_period": "10",
            "batch_max_element": "100",
            "observe_ethereum_height_period": "50",
        }
    }

    # migrate to sdk v0.46
    custom_cronos.supervisorctl("stop", "all")
    sdk_version = "v0.46"
    file_path1 = Path(migrate / f"{sdk_version}.json")
    with open(file_path1, "w") as fp:
        json.dump(cli.migrate_sdk_genesis(sdk_version, str(file_path0)), fp)
        fp.flush()
    # migrate to cronos v1.0.x
    cronos_version = "v1.0"
    file_path2 = Path(migrate / f"{cronos_version}.json")
    with open(file_path2, "w") as fp:
        json.dump(cli.migrate_cronos_genesis(cronos_version, str(file_path1)), fp)
        fp.flush()
    print(cli.validate_genesis(str(file_path2)))

    # update the genesis time = current time + 5 secs
    newtime = datetime.utcnow() + timedelta(seconds=5)
    newtime = newtime.replace(tzinfo=None).isoformat("T") + "Z"
    config = custom_cronos.config
    config["genesis-time"] = newtime
    for i, _ in enumerate(config["validators"]):
        genesis = json.load(open(file_path2))
        genesis["genesis_time"] = config.get("genesis-time")
        file = custom_cronos.cosmos_cli(i).data_dir / "config/genesis.json"
        file.write_text(json.dumps(genesis))
    custom_cronos.supervisorctl("start", "cronos_777-1-node0", "cronos_777-1-node1")
    wait_for_new_blocks(custom_cronos.cosmos_cli(), 1)
    custom_cronos.supervisorctl("stop", "all")
