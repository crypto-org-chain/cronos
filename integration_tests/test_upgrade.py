import json
import shutil
import stat
import subprocess
import time
from contextlib import contextmanager
from datetime import datetime, timedelta
from pathlib import Path

import pytest
import requests
from pystarport import ports
from pystarport.cluster import SUPERVISOR_CONFIG_FILE
from hexbytes import HexBytes
from web3 import exceptions

from .network import Cronos, setup_custom_cronos
from .utils import (
    ADDRS,
    CONTRACTS,
    approve_proposal,
    assert_gov_params,
    deploy_contract,
    edit_ini_sections,
    eth_to_bech32,
    get_consensus_params,
    get_send_enable,
    send_transaction,
    wait_for_block,
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

    def do_upgrade(plan_name, target, mode=None):
        print(f"upgrade {plan_name} height: {target}")
        if plan_name in ("v1.5", "v1.6", "v1.7"):
            rsp = cli.submit_gov_proposal(
                "community",
                "software-upgrade",
                {
                    "name": plan_name,
                    "title": "upgrade test",
                    "note": "ditto",
                    "upgrade-height": target,
                    "summary": "summary",
                    "deposit": "10000basetcro",
                },
                broadcast_mode="sync",
            )
            assert rsp["code"] == 0, rsp["raw_log"]
            approve_proposal(
                c, rsp["events"], msg="/cosmos.upgrade.v1beta1.MsgSoftwareUpgrade"
            )
        else:
            rsp = cli.gov_propose_legacy(
                "community",
                "software-upgrade",
                {
                    "name": plan_name,
                    "title": "upgrade test",
                    "description": "ditto",
                    "upgrade-height": target,
                    "deposit": "10000basetcro",
                },
                mode=mode,
            )
            assert rsp["code"] == 0, rsp["raw_log"]
            approve_proposal(
                c,
                rsp["logs"][0]["events"],
                msg="/cosmos.upgrade.v1beta1.MsgSoftwareUpgrade",
                wait_tx=False,
            )

        # update cli chain binary
        c.chain_binary = (
            Path(c.chain_binary).parent.parent.parent / f"{plan_name}/bin/cronosd"
        )
        # block should pass the target height
        wait_for_block(c.cosmos_cli(), target + 2, timeout=480)
        wait_for_port(ports.rpc_port(base_port))
        return c.cosmos_cli()

    # test migrate keystore
    cli.migrate_keystore()
    height = cli.block_height()
    target_height0 = height + 15
    cli = do_upgrade("v1.1.0", target_height0, "block")
    check_basic_tx(c)

    height = cli.block_height()
    target_height1 = height + 15

    w3 = c.w3
    random_contract = deploy_contract(
        w3,
        CONTRACTS["Random"],
    )
    with pytest.raises(exceptions.Web3RPCError) as e_info:
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

    cli = do_upgrade("v1.2", target_height1)
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
    cli = do_upgrade("v1.3", height + 15)
    assert txs == get_txs(base_port, height)

    gov_param = cli.query_params("gov")

    c.supervisorctl("stop", "cronos_777-1-node0")
    time.sleep(3)
    cli.changeset_fixdata(f"{c.base_dir}/node0/data/versiondb")
    print(cli.changeset_fixdata(f"{c.base_dir}/node0/data/versiondb", dry_run=True))
    c.supervisorctl("start", "cronos_777-1-node0")
    wait_for_port(ports.evmrpc_port(c.base_port(0)))

    to = "0x2D5B6C193C39D2AECb4a99052074E6F325258a0f"
    with pytest.raises(AssertionError) as err:
        cli.query_account(eth_to_bech32(to))
    assert "crc194dkcxfu88f2aj62nyzjqa8x7vjjtzs0jwcj06 not found" in str(err.value)
    receipt = send_transaction(w3, {"to": to, "value": 10, "gas": 21000})
    method = "debug_traceTransaction"
    params = [receipt["transactionHash"].hex(), {"tracer": "callTracer"}]
    tx_bf = w3.provider.make_request(method, params)

    cli = do_upgrade("v1.4", cli.block_height() + 15)

    assert_evm_params(cli, e0, target_height0 - 1)
    assert_evm_params(cli, e1, target_height1 - 1)
    assert f0 == cli.query_params("feemarket", height=target_height0 - 1)
    assert f1 == cli.query_params("feemarket", height=target_height1 - 1)
    assert cli.query_params("evm")["header_hash_num"] == "256", p
    with pytest.raises(AssertionError):
        cli.query_params("icaauth")
    assert_gov_params(cli, gov_param)

    cli = do_upgrade("v1.5", cli.block_height() + 15)
    check_basic_tx(c)

    tx_af = w3.provider.make_request(method, params)
    assert tx_af.get("result") == tx_bf.get("result"), tx_af

    cli = do_upgrade("v1.6", cli.block_height() + 15)
    check_basic_tx(c)

    tx_af = w3.provider.make_request(method, params)
    assert tx_af.get("result") == tx_bf.get("result"), tx_af

    cli = do_upgrade("v1.7", cli.block_height() + 15)
    check_basic_tx(c)

    tx_af = w3.provider.make_request(method, params)
    assert tx_af.get("result") == tx_bf.get("result"), tx_af

    # check preinstall correctly installed
    historical_storage_address = "0x0000F90827F1C53a10cb7A02335B175320002935"
    expected_historical_storage_address_code = (
        "3373fffffffffffffffffffffffffffffffffffffffe14604657602036036042575f356001"
        "43038111604257611fff81430311604257611fff9006545f5260205ff35b5f5ffd5b5f35611fff60014303065500"
    )
    historical_storage_address_code = w3.eth.get_code(historical_storage_address)
    assert historical_storage_address_code == HexBytes(
        expected_historical_storage_address_code
    )


def test_cosmovisor_upgrade(custom_cronos: Cronos, tmp_path_factory):
    exec(custom_cronos, tmp_path_factory)
