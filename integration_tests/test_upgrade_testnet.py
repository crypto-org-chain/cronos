import json
import subprocess
from datetime import datetime, timedelta
from pathlib import Path

import pytest
from pystarport import ports

from .network import Cronos, setup_custom_cronos
from .upgrade_utils import post_init
from .utils import (
    ADDRS,
    CONTRACTS,
    approve_proposal,
    deploy_contract,
    get_consensus_params,
    send_transaction,
    wait_for_block,
    wait_for_new_blocks,
    wait_for_port,
)


@pytest.fixture(scope="module")
def custom_cronos(tmp_path_factory):
    path = tmp_path_factory.mktemp("upgrade")
    cmd = [
        "nix-build",
        Path(__file__).parent / "configs/upgrade-testnet-test-package.nix",
        "-o",
        path / "upgrades",
    ]
    print(*cmd)
    subprocess.run(cmd, check=True)
    # init with genesis binary
    yield from setup_custom_cronos(
        path,
        26200,
        Path(__file__).parent / "configs/cosmovisor_testnet.jsonnet",
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
    port = ports.api_port(custom_cronos.base_port(0))
    send_enable = [
        {"denom": "basetcro", "enabled": False},
        {"denom": "stake", "enabled": True},
    ]
    p = cli.query_bank_send()
    assert sorted(p, key=lambda x: x["denom"]) == send_enable

    # export genesis from cronos v1.1.0-rc1
    custom_cronos.supervisorctl("stop", "all")
    migrate = tmp_path_factory.mktemp("migrate")
    file_path0 = Path(migrate / "v1.1.0-rc1.json")
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

    plan_name = "v1.1.0-testnet"
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
        mode=None,
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    approve_proposal(custom_cronos, rsp, False)

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
    # check consensus params
    port = ports.rpc_port(custom_cronos.base_port(0))
    res = get_consensus_params(port, w3.eth.get_block_number())
    assert res["block"]["max_gas"] == "60000000"

    # check bank send enable
    p = cli.query_bank_send()
    assert sorted(p, key=lambda x: x["denom"]) == send_enable

    rsp = cli.query_params("icaauth")
    assert rsp["params"]["min_timeout_duration"] == "3600s", rsp
    max_callback_gas = cli.query_params()["max_callback_gas"]
    assert max_callback_gas == "50000", max_callback_gas

    # update the genesis time = current time + 5 secs
    newtime = datetime.utcnow() + timedelta(seconds=5)
    newtime = newtime.replace(tzinfo=None).isoformat("T") + "Z"
    config = custom_cronos.config
    config["genesis-time"] = newtime
    for i, _ in enumerate(config["validators"]):
        genesis = json.load(open(file_path0))
        genesis["genesis_time"] = config.get("genesis-time")
        file = custom_cronos.cosmos_cli(i).data_dir / "config/genesis.json"
        file.write_text(json.dumps(genesis))
    custom_cronos.supervisorctl("start", "cronos_777-1-node0", "cronos_777-1-node1")
    wait_for_new_blocks(custom_cronos.cosmos_cli(), 1)
    custom_cronos.supervisorctl("stop", "all")
