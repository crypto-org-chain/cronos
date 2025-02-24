import json
import shutil
import stat
import subprocess
import time
from contextlib import contextmanager
from datetime import datetime, timedelta
from pathlib import Path

import pytest
from pystarport import ports
from pystarport.cluster import SUPERVISOR_CONFIG_FILE

from .cosmoscli import DEFAULT_GAS_PRICE
from .network import Cronos, setup_custom_cronos
from .utils import (
    ADDRS,
    CONTRACTS,
    KEYS,
    approve_proposal,
    deploy_contract,
    edit_ini_sections,
    get_consensus_params,
    get_send_enable,
    send_transaction,
    sign_transaction,
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
            "command": f"cosmovisor run start --home %(here)s/node{i}",
            "environment": (
                "DAEMON_NAME=cronosd,"
                "DAEMON_SHUTDOWN_GRACE=1m,"
                "UNSAFE_SKIP_BACKUP=true,"
                f"DAEMON_HOME=%(here)s/node{i}"
            ),
        },
    )


def setup_cronos_test(tmp_path_factory):
    path = tmp_path_factory.mktemp("upgrade")
    port = 26200
    nix_name = "upgrade-test-package"
    cfg_name = "cosmovisor"
    configdir = Path(__file__).parent
    cmd = [
        "nix-build",
        configdir / f"configs/{nix_name}.nix",
    ]
    print(*cmd)
    subprocess.run(cmd, check=True)

    # copy the content so the new directory is writable.
    upgrades = path / "upgrades"
    shutil.copytree("./result", upgrades)
    mod = stat.S_IRWXU
    upgrades.chmod(mod)
    for d in upgrades.iterdir():
        d.chmod(mod)

    # init with genesis binary
    with contextmanager(setup_custom_cronos)(
        path,
        port,
        configdir / f"configs/{cfg_name}.jsonnet",
        post_init=post_init,
        chain_binary=str(upgrades / "genesis/bin/cronosd"),
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
    w3 = c.w3
    gas_price = w3.eth.gas_price
    erc20 = deploy_contract(
        w3,
        CONTRACTS["TestERC20A"],
        key=KEYS["validator"],
        gas_price=gas_price,
    )
    tx = erc20.functions.transfer(ADDRS["community"], 10).build_transaction(
        {
            "from": ADDRS["validator"],
            "gasPrice": gas_price,
        }
    )
    signed = sign_transaction(w3, tx, KEYS["validator"])
    txhash0 = w3.eth.send_raw_transaction(signed.rawTransaction)
    receipt0 = w3.eth.wait_for_transaction_receipt(txhash0)
    block0 = hex(receipt0["blockNumber"])
    logs0 = w3.eth.get_logs({"fromBlock": block0, "toBlock": block0})

    def assert_eth_call():
        rsp = w3.eth.call(
            {
                "from": ADDRS["validator"],
                "to": erc20.address,
                "data": tx["data"],
            },
            block0,
        )
        assert (1,) == w3.codec.decode(("uint256",), rsp)

    assert_eth_call()
    send_enable = [
        {"denom": "basetcro", "enabled": False},
        {"denom": "stake", "enabled": True},
    ]
    p = get_send_enable(port)
    assert sorted(p, key=lambda x: x["denom"]) == send_enable

    # export genesis from old version
    c.supervisorctl("stop", "all")
    migrate = tmp_path_factory.mktemp("migrate")
    file_path0 = Path(migrate / "old.json")
    with open(file_path0, "w") as fp:
        json.dump(json.loads(cli.export()), fp)
        fp.flush()

    c.supervisorctl("start", "cronos_777-1-node0", "cronos_777-1-node1")
    wait_for_port(ports.evmrpc_port(c.base_port(0)))
    wait_for_new_blocks(cli, 1)

    def do_upgrade(
        plan_name,
        target,
        mode=None,
        method="submit-legacy-proposal",
        gas_prices=DEFAULT_GAS_PRICE,
    ):
        rsp = cli.gov_propose_legacy(
            "community",
            "software-upgrade",
            {
                "name": plan_name,
                "title": "upgrade test",
                "description": "ditto",
                "upgrade-height": target,
                "deposit": "1basetcro",
            },
            mode=mode,
            method=method,
            gas_prices=gas_prices,
        )
        assert rsp["code"] == 0, rsp["raw_log"]
        approve_proposal(c, rsp, gas_prices=gas_prices)

        # update cli chain binary
        c.chain_binary = (
            Path(c.chain_binary).parent.parent.parent / f"{plan_name}/bin/cronosd"
        )
        # block should pass the target height
        wait_for_block(c.cosmos_cli(), target + 2, timeout=480)
        wait_for_port(ports.rpc_port(c.base_port(0)))

    target1 = cli.block_height() + 15
    print("upgrade v0.8.0 height", target1)
    gas_prices = "5000000000000basetcro"
    do_upgrade(
        "v0.7.0-hotfix",
        target1,
        "block",
        method="submit-proposal",
        gas_prices=gas_prices,
    )
    cli = c.cosmos_cli()

    target2 = cli.block_height() + 15
    print("upgrade v1.0 height", target2)
    do_upgrade(
        "v1.0.0",
        target2,
        "block",
        method="submit-proposal",
        gas_prices=gas_prices,
    )
    cli = c.cosmos_cli()

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

    # test migrate keystore
    cli.migrate_keystore()
    target3 = cli.block_height() + 15
    print("upgrade v1.1 height", target3)

    do_upgrade("v1.1.0", target3, "block", gas_prices=gas_prices)
    cli = c.cosmos_cli()

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

    target4 = cli.block_height() + 15
    print("upgrade v1.3 height", target4)
    do_upgrade("v1.3", target4, gas_prices=gas_prices)
    cli = c.cosmos_cli()

    c.supervisorctl("stop", "cronos_777-1-node0")
    time.sleep(3)
    cli.changeset_fixdata(f"{c.base_dir}/node0/data/versiondb")
    assert not cli.changeset_fixdata(f"{c.base_dir}/node0/data/versiondb", dry_run=True)
    c.supervisorctl("start", "cronos_777-1-node0")
    wait_for_port(ports.evmrpc_port(c.base_port(0)))

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

    e0 = cli.query_params("evm", height=target3 - 1)["params"]
    e1 = cli.query_params("evm", height=target4 - 1)["params"]
    f0 = cli.query_params("feemarket", height=target3 - 1)["params"]
    f1 = cli.query_params("feemarket", height=target4 - 1)["params"]
    assert e0["evm_denom"] == e1["evm_denom"] == "basetcro"

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

    assert e0 == cli.query_params("evm", height=target3 - 1)["params"]
    assert e1 == cli.query_params("evm", height=target4 - 1)["params"]
    assert f0 == cli.query_params("feemarket", height=target3 - 1)["params"]
    assert f1 == cli.query_params("feemarket", height=target4 - 1)["params"]

    assert w3.eth.wait_for_transaction_receipt(txhash0)["logs"] == receipt0["logs"]
    assert w3.eth.get_logs({"fromBlock": block0, "toBlock": block0}) == logs0
    assert_eth_call()


def test_cosmovisor_upgrade(custom_cronos: Cronos, tmp_path_factory):
    exec(custom_cronos, tmp_path_factory)
