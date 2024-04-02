import json
import subprocess
from contextlib import contextmanager
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
    get_consensus_params,
    send_transaction,
    wait_for_block,
    wait_for_new_blocks,
    wait_for_port,
)

pytestmark = pytest.mark.upgrade


@pytest.fixture(scope="module")
def custom_cronos(tmp_path_factory):
    yield from setup_cronos_test(tmp_path_factory)


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


def setup_cronos_test(tmp_path_factory):
    path = tmp_path_factory.mktemp("upgrade")
    port = 26200
    nix_name = "upgrade-test-package"
    cfg_name = "cosmovisor"
    cmd = [
        "nix-build",
        Path(__file__).parent / f"configs/{nix_name}.nix",
        "-o",
        path / "upgrades",
    ]
    print(*cmd)
    subprocess.run(cmd, check=True)
    # init with genesis binary
    with contextmanager(setup_custom_cronos)(
        path,
        port,
        Path(__file__).parent / f"configs/{cfg_name}.jsonnet",
        post_init=post_init,
        chain_binary=str(path / "upgrades/genesis/bin/cronosd"),
    ) as cronos:
        yield cronos


def exec(c, tmp_path_factory):
    """
    - propose an upgrade and pass it
    - wait for it to happen
    - it should work transparently
    """
    cli = c.cosmos_cli()
    port = ports.api_port(c.base_port(0))
    send_enable = [
        {"denom": "basetcro", "enabled": False},
        {"denom": "stake", "enabled": True},
    ]
    p = cli.query_bank_send()
    assert sorted(p, key=lambda x: x["denom"]) == send_enable

    # export genesis from old version
    c.supervisorctl("stop", "all")
    migrate = tmp_path_factory.mktemp("migrate")
    file_path0 = Path(migrate / "old.json")
    cli.export(output_document=str(file_path0))

    c.supervisorctl("start", "cronos_777-1-node0", "cronos_777-1-node1")
    wait_for_port(ports.evmrpc_port(c.base_port(0)))
    wait_for_new_blocks(cli, 1)

    height = cli.block_height()
    target_height = height + 15
    print("upgrade height", target_height)

    w3 = c.w3
    random_contract = deploy_contract(
        c.w3,
        CONTRACTS["Random"],
    )
    with pytest.raises(ValueError) as e_info:
        random_contract.caller.randomTokenId()
    assert "invalid memory address or nil pointer dereference" in str(e_info.value)
    contract = deploy_contract(w3, CONTRACTS["TestERC20A"])
    old_height = w3.eth.block_number
    old_balance = w3.eth.get_balance(ADDRS["validator"], block_identifier=old_height)
    old_base_fee = w3.eth.get_block(old_height).baseFeePerGas
    old_erc20_balance = contract.caller(block_identifier=old_height).balanceOf(
        ADDRS["validator"]
    )
    print("old values", old_height, old_balance, old_base_fee)

    plan_name = "v1.2"
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
    approve_proposal(c, rsp, event_query_tx=True)

    # update cli chain binary
    c.chain_binary = (
        Path(c.chain_binary).parent.parent.parent / f"{plan_name}/bin/cronosd"
    )
    cli = c.cosmos_cli()

    # block should pass the target height
    wait_for_block(cli, target_height + 2, timeout=480)
    wait_for_port(ports.rpc_port(c.base_port(0)))

    # test migrate keystore
    cli.migrate_keystore()

    # check basic tx works
    wait_for_port(ports.evmrpc_port(c.base_port(0)))
    receipt = send_transaction(
        c.w3,
        {
            "to": ADDRS["community"],
            "value": 1000,
            "maxFeePerGas": 10000000000000,
            "maxPriorityFeePerGas": 10000,
        },
    )
    assert receipt.status == 1

    # deploy contract should still work
    deploy_contract(w3, CONTRACTS["Greeter"])
    # random should work
    res = random_contract.caller.randomTokenId()
    assert res > 0, res

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
    port = ports.rpc_port(c.base_port(0))
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
    config = c.config
    config["genesis-time"] = newtime
    for i, _ in enumerate(config["validators"]):
        genesis = json.load(open(file_path0))
        genesis["genesis_time"] = config.get("genesis-time")
        file = c.cosmos_cli(i).data_dir / "config/genesis.json"
        file.write_text(json.dumps(genesis))
    c.supervisorctl("start", "cronos_777-1-node0", "cronos_777-1-node1")
    wait_for_new_blocks(c.cosmos_cli(), 1)
    c.supervisorctl("stop", "all")


def test_cosmovisor_upgrade(custom_cronos: Cronos, tmp_path_factory):
    exec(custom_cronos, tmp_path_factory)
