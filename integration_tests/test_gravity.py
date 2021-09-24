import json
from pathlib import Path

import pytest
from eth_account.account import Account
from hexbytes import HexBytes
from mnemonic import Mnemonic
from pystarport import ports

from .conftest import setup_cronos, setup_geth
from .network import GravityBridge
from .utils import (
    ADDRS,
    KEYS,
    add_ini_sections,
    cronos_address_from_mnemonics,
    deploy_contract,
    send_transaction,
    sign_validator,
    supervisorctl,
    wait_for_fn,
    wait_for_new_blocks,
)

pytestmark = pytest.mark.gravity

Account.enable_unaudited_hdwallet_features()


def cronos_crc20_abi():
    path = Path(__file__).parent.parent / "x/cronos/types/contracts/ModuleCRC20.json"
    return json.load(path.open())["abi"]


@pytest.fixture(scope="module")
def geth(tmp_path_factory):
    "start-geth"
    for network in setup_geth(tmp_path_factory.mktemp("geth"), 8555):
        yield network.w3


@pytest.fixture(scope="module")
def cronos(tmp_path_factory):
    "start-cronos"
    yield from setup_cronos(tmp_path_factory.mktemp("cronos"), 26700)


@pytest.fixture(scope="module")
def gravity(cronos, geth, suspend_capture):
    """
    - set-delegator-keys
    - deploy gravity contract
    - start orchestrator
    """
    chain_id = "cronos_777-1"
    mnemonic_gen = Mnemonic("english")

    # set-delegate-keys
    eth_accounts = []  # eth accounts created for orchestrators
    cosmos_mnemonics = []  # cosmos mnemonics created for orchestrators
    for i, val in enumerate(cronos.config["validators"]):
        # generate orchestrator eth key
        acct = Account.create()
        eth_accounts.append(acct)

        # fund the orchestrator account in geth
        send_transaction(
            geth, {"to": acct.address, "value": 10 ** 17}, KEYS["validator"]
        )

        # orchestrator cronos key
        mnemonic = mnemonic_gen.generate()
        cosmos_mnemonics.append(mnemonic)
        acc_addr = cronos_address_from_mnemonics(mnemonic)

        # fund the orchestrator account in cronos
        cli = cronos.cosmos_cli(i)
        cli.transfer("validator", acc_addr, "%dbasetcro" % 10 ** 16)

        val_addr = cli.address("validator", bech="val")
        val_acct_addr = cli.address("validator")
        nonce = int(cli.account(val_acct_addr)["base_account"]["sequence"])
        signature = sign_validator(acct, val_addr, nonce)
        rsp = cli.set_delegate_keys(
            val_addr, acc_addr, acct.address, signature, from_=val_acct_addr
        )
        assert rsp["code"] == 0, rsp["raw_log"]
    # wait for gravity signer tx get generated
    wait_for_new_blocks(cli, 2)

    # deploy gravity contract to geth
    gravity_id = cli.query_gravity_params()["params"]["gravity_id"]
    # hardcode the nonce as 3, because we can't query latest directly
    # https://github.com/PeggyJV/gravity-bridge/issues/154
    signer_set = cli.query_signer_set_tx(3)["signer_set"]["signers"]
    powers = [int(signer["power"]) for signer in signer_set]
    threshold = int(2 ** 32 * 0.66)  # gravity normalize the power to [0, 2**32]
    eth_addresses = [signer["ethereum_address"] for signer in signer_set]
    assert sum(powers) >= threshold, "not enough validator on board"

    contract = deploy_contract(
        geth,
        Path(__file__).parent
        / "contracts/artifacts/contracts/Gravity.sol/Gravity.json",
        (gravity_id.encode(), threshold, eth_addresses, powers),
    )
    print("contract deployed", contract.address)

    # start orchestrator:
    # a) add process into the supervisord config file
    # b) reload supervisord
    programs = {}
    for i, val in enumerate(cronos.config["validators"]):
        metrics_port = 3000 + i
        grpc_port = ports.grpc_port(val["base_port"])
        cmd = (
            f'orchestrator --cosmos-phrase="{cosmos_mnemonics[i]}" '
            f"--ethereum-key={eth_accounts[i].key.hex()} "
            f"--cosmos-grpc=http://localhost:{grpc_port} "
            f"--ethereum-rpc={geth.provider.endpoint_uri} "
            "--address-prefix=crc --fees=basetcro "
            f"--contract-address={contract.address} "
            f"--metrics-listen 127.0.0.1:{metrics_port} "
            f'''--hd-wallet-path="m/44'/60'/0'/0/0"'''
        )
        programs[f"program:{chain_id}-orchestrator{i}"] = {
            "command": cmd,
            "autostart": "true",
            "autorestart": "true",
            "startsecs": "3",
            "redirect_stderr": "true",
            "stdout_logfile": f"%(here)s/orchestrator{i}.log",
        }

    add_ini_sections(cronos.base_dir / "tasks.ini", programs)
    supervisorctl(cronos.base_dir / "../tasks.ini", "update")

    yield GravityBridge(cronos, geth, contract)


def test_gravity_transfer(gravity, suspend_capture):
    geth = gravity.geth
    cli = gravity.cronos.cosmos_cli()
    cronos_w3 = gravity.cronos.w3

    # deploy test erc20 contract
    erc20 = deploy_contract(
        geth,
        Path(__file__).parent
        / "contracts/artifacts/contracts/TestERC20A.sol/TestERC20A.json",
    )

    balance = erc20.caller.balanceOf(geth.eth.coinbase)
    assert balance == 100000000000000000000000000
    amount = 1000

    tx_tpl = {"from": geth.eth.coinbase}

    # approve
    txhash = erc20.functions.approve(gravity.contract.address, amount).transact(tx_tpl)
    geth.eth.wait_for_transaction_receipt(txhash)

    # gravity send
    print("send to cronos crc20")
    recipient = HexBytes(ADDRS["community"])
    txhash = gravity.contract.functions.sendToCosmos(
        erc20.address, b"\x00" * 12 + recipient, amount
    ).transact(tx_tpl)
    geth.eth.wait_for_transaction_receipt(txhash)
    assert erc20.caller.balanceOf(geth.eth.coinbase) == balance - amount

    denom = "gravity" + erc20.address

    crc20_contract = None

    def check():
        nonlocal crc20_contract
        try:
            rsp = cli.query_contract_by_denom(denom)
        except AssertionError:
            # not deployed yet
            return False
        assert len(rsp["auto_contract"]) > 0
        crc20_contract = cronos_w3.eth.contract(
            address=rsp["auto_contract"], abi=cronos_crc20_abi()
        )
        return crc20_contract.caller.balanceOf(recipient) == amount

    wait_for_fn("send-to-cronos", check)

    # send it back to erc20
    tx = crc20_contract.functions.send_to_ethereum(
        geth.eth.coinbase, amount, 0
    ).buildTransaction(
        {
            "gas": 42000,
            "nonce": cronos_w3.eth.get_transaction_count(recipient),
        }
    )
    txreceipt = send_transaction(cronos_w3, tx, KEYS["community"])
    assert txreceipt.status == 1, "should success"

    def check():
        v = erc20.caller.balanceOf(geth.eth.coinbase)
        return v == balance

    wait_for_fn("send-to-ethereum", check)
