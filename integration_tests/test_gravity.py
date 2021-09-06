import time
from pathlib import Path

import pytest
from eth_account.account import Account
from pystarport import ports

from .conftest import setup_cronos, setup_geth
from .network import GravityBridge
from .utils import (
    add_ini_sections,
    decode_bech32,
    deploy_contract,
    sign_validator,
    supervisorctl,
    wait_for_fn,
    wait_for_new_blocks,
)

pytestmark = pytest.mark.gravity

Account.enable_unaudited_hdwallet_features()


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
    # set-delegate-keys
    for i, val in enumerate(cronos.config["validators"]):
        # use the same key for cronos validator, geth, orchestrator
        cli = cronos.cosmos_cli(i)
        val_addr = cli.address("validator", bech="val")
        acc_addr = cli.address("validator")
        nonce = int(cli.account(acc_addr)["base_account"]["sequence"])
        acct = Account.from_mnemonic(val["mnemonic"])
        signature = sign_validator(acct, val_addr, nonce)
        rsp = cli.set_delegate_keys(val_addr, acc_addr, acct.address, signature)
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
        mnemonic = val["mnemonic"]
        acct = Account.from_mnemonic(mnemonic)

        # fund the address in geth
        geth.eth.wait_for_transaction_receipt(
            geth.eth.send_transaction(
                {"from": geth.eth.coinbase, "to": acct.address, "value": 10 ** 17}
            )
        )

        metrics_port = 3000 + i
        grpc_port = ports.grpc_port(val["base_port"])
        cmd = (
            f'orchestrator --cosmos-phrase="{mnemonic}" '
            f"--ethereum-key={acct.key.hex()} "
            f"--cosmos-grpc=http://localhost:{grpc_port} "
            f"--ethereum-rpc={geth.provider.endpoint_uri} "
            "--address-prefix=eth --fees=basetcro "
            f"--contract-address={contract.address} "
            f"--metrics-listen 127.0.0.1:{metrics_port}"
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
    print("send to cronos")
    cosmos_recipient = cli.address("community")
    recipient = b"\x00" * 12 + decode_bech32(cosmos_recipient)
    txhash = gravity.contract.functions.sendToCosmos(
        erc20.address, recipient, amount
    ).transact(tx_tpl)
    geth.eth.wait_for_transaction_receipt(txhash)
    assert erc20.caller.balanceOf(geth.eth.coinbase) == balance - amount

    denom = "gravity" + erc20.address

    def check():
        return cli.balance(cosmos_recipient, denom) == amount

    wait_for_fn("send-to-cosmos", check)

    # send it back to erc20
    print("send back to ethereum")
    rsp = cli.send_to_ethereum(
        cosmos_recipient, geth.eth.coinbase, f"{amount}{denom}", f"0{denom}"
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    assert cli.balance(cosmos_recipient, denom) == 0

    def check():
        return erc20.caller.balanceOf(geth.eth.coinbase) == balance

    wait_for_fn("send-to-ethereum", check)
