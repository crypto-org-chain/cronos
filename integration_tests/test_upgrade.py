import json
import shutil
import stat
import subprocess
from contextlib import contextmanager
from datetime import datetime, timedelta
from pathlib import Path

import pytest
import requests
from pystarport import ports

from .network import Cronos, setup_custom_cronos
from .utils import (
    ADDRS,
    CONTRACTS,
    assert_gov_params,
    deploy_contract,
    do_upgrade,
    get_consensus_params,
    get_send_enable,
    post_init,
    send_transaction,
    wait_for_new_blocks,
    wait_for_port,
)

pytestmark = pytest.mark.upgrade


@pytest.fixture(scope="module")
def custom_cronos(tmp_path_factory):
    yield from setup_cronos_test(tmp_path_factory)


def get_txs(base_port, end):
    port = ports.rpc_port(base_port)
    res = []
    for h in range(1, end):
        url = f"http://127.0.0.1:{port}/block_results?height={h}"
        res.append(requests.get(url).json().get("result")["txs_results"])
    return res


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


def assert_evm_params(cli, expected, height):
    params = cli.query_params("evm", height=height)
    del params["header_hash_num"]
    assert expected == params


def check_basic_tx(c):
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


def exec(c, tmp_path_factory):
    """
    - propose an upgrade and pass it
    - wait for it to happen
    - it should work transparently
    """
    cli = c.cosmos_cli()
    base_port = c.base_port(0)
    port = ports.api_port(base_port)
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

    c.supervisorctl(
        "start", "cronos_777-1-node0", "cronos_777-1-node1", "cronos_777-1-node2"
    )
    wait_for_port(ports.evmrpc_port(base_port))
    wait_for_new_blocks(cli, 1)

    # test migrate keystore
    cli.migrate_keystore()
    height = cli.block_height()
    target_height0 = height + 15
    cli = do_upgrade(c, "v1.1.0", target_height0, "block")
    check_basic_tx(c)

    height = cli.block_height()
    target_height1 = height + 15

    w3 = c.w3
    random_contract = deploy_contract(
        w3,
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

    cli = do_upgrade(c, "v1.2", target_height1)
    check_basic_tx(c)

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
    port = ports.rpc_port(base_port)
    res = get_consensus_params(port, w3.eth.get_block_number())
    assert res["block"]["max_gas"] == "60000000"

    # check bank send enable
    p = cli.query_bank_send()
    assert sorted(p, key=lambda x: x["denom"]) == send_enable

    rsp = cli.query_params("icaauth")
    assert rsp["min_timeout_duration"] == "3600s", rsp
    max_callback_gas = cli.query_params()["max_callback_gas"]
    assert max_callback_gas == "50000", max_callback_gas

    e0 = cli.query_params("evm", height=target_height0 - 1)
    e1 = cli.query_params("evm", height=target_height1 - 1)
    f0 = cli.query_params("feemarket", height=target_height0 - 1)
    f1 = cli.query_params("feemarket", height=target_height1 - 1)
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
    c.supervisorctl(
        "start", "cronos_777-1-node0", "cronos_777-1-node1", "cronos_777-1-node2"
    )
    wait_for_new_blocks(c.cosmos_cli(), 1)

    height = cli.block_height()
    txs = get_txs(base_port, height)
    cli = do_upgrade(c, "v1.3", height + 15)
    assert txs == get_txs(base_port, height)

    gov_param = cli.query_params("gov")
    cli = do_upgrade(c, "v1.4", cli.block_height() + 15)

    assert_evm_params(cli, e0, target_height0 - 1)
    assert_evm_params(cli, e1, target_height1 - 1)
    assert f0 == cli.query_params("feemarket", height=target_height0 - 1)
    assert f1 == cli.query_params("feemarket", height=target_height1 - 1)
    assert cli.query_params("evm")["header_hash_num"] == "256", p
    with pytest.raises(AssertionError):
        cli.query_params("icaauth")
    assert_gov_params(cli, gov_param)

    cli = do_upgrade(c, "v1.4.0-rc5-testnet", cli.block_height() + 15)
    check_basic_tx(c)


def test_cosmovisor_upgrade(custom_cronos: Cronos, tmp_path_factory):
    exec(custom_cronos, tmp_path_factory)
