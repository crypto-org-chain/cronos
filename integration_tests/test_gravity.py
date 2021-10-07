import json
import subprocess
from pathlib import Path

import pytest
import toml
from dateutil.parser import isoparse
from eth_account.account import Account
from hexbytes import HexBytes
from mnemonic import Mnemonic
from pystarport import ports

from .conftest import setup_cronos, setup_geth
from .network import GravityBridge
from .utils import (
    ADDRS,
    KEYS,
    InlineTable,
    add_ini_sections,
    cronos_address_from_mnemonics,
    deploy_contract,
    parse_events,
    send_to_cosmos,
    send_transaction,
    sign_validator,
    supervisorctl,
    wait_for_block_time,
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
        print("fund 0.1 eth to address", acct.address)
        send_transaction(
            geth, {"to": acct.address, "value": 10 ** 17}, KEYS["validator"]
        )

        # orchestrator cronos key
        mnemonic = mnemonic_gen.generate(strength=256)
        cosmos_mnemonics.append(mnemonic)
        acc_addr = cronos_address_from_mnemonics(mnemonic)

        # fund the orchestrator account in cronos
        print("fund 100cro to address", acc_addr)
        rsp = cronos.cosmos_cli().transfer(
            "community", acc_addr, "%dbasetcro" % (100 * (10 ** 18))
        )
        assert rsp["code"] == 0, rsp["raw_log"]

        cli = cronos.cosmos_cli(i)
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
    signer_set = cli.query_latest_signer_set_tx()["signer_set"]["signers"]
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
    print("gravity contract deployed", contract.address)

    # start orchestrator:
    # a) add process into the supervisord config file
    # b) reload supervisord
    programs = {}
    for i, val in enumerate(cronos.config["validators"]):
        metrics_port = 3000 + i
        grpc_port = ports.grpc_port(val["base_port"])
        gorc_config = cronos.base_dir / f"node{i}/gorc.toml"

        # generate gorc config file
        gorc_config.write_text(
            toml.dumps(
                {
                    "keystore": InlineTable(
                        {
                            "File": str(
                                cronos.base_dir / f"node{i}/orchestrator_keystore"
                            ),
                        }
                    ),
                    "gravity": {
                        "contract": contract.address,
                        "fees_denom": "basetcro",
                    },
                    "ethereum": {
                        "key_derivation_path": "m/44'/60'/0'/0/0",
                        "rpc": geth.provider.endpoint_uri,
                    },
                    "cosmos": {
                        "gas_price": {
                            "amount": 5000000000000,
                            "denom": "basetcro",
                        },
                        "grpc": f"http://localhost:{grpc_port}",
                        "key_derivation_path": "m/44'/60'/0'/0/0",
                        "prefix": "crc",
                    },
                    "metrics": {
                        "listen_addr": f"127.0.0.1:{metrics_port}",
                    },
                },
                encoder=toml.TomlPreserveInlineDictEncoder(),
            )
        )

        # load keys
        subprocess.run(
            [
                "gorc",
                "-c",
                gorc_config,
                "keys",
                "eth",
                "recover",
                "cosmos",
                cosmos_mnemonics[i],
            ],
            check=True,
        )
        subprocess.run(
            [
                "gorc",
                "-c",
                gorc_config,
                "keys",
                "eth",
                "import",
                "eth",
                eth_accounts[i].key.hex(),
            ],
            check=True,
        )

        programs[f"program:{chain_id}-orchestrator{i}"] = {
            "command": (
                f'gorc -c "{gorc_config}" orchestrator start '
                "--cosmos-key cosmos --ethereum-key eth"
            ),
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

    balance = erc20.caller.balanceOf(ADDRS["validator"])
    assert balance == 100000000000000000000000000
    amount = 1000

    print("send to cronos crc20")
    recipient = HexBytes(ADDRS["community"])
    txreceipt = send_to_cosmos(
        gravity.contract, erc20, recipient, amount, KEYS["validator"]
    )
    assert erc20.caller.balanceOf(ADDRS["validator"]) == balance - amount

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
        ADDRS["validator"], amount, 0
    ).buildTransaction({"from": ADDRS["community"]})
    txreceipt = send_transaction(cronos_w3, tx, KEYS["community"])
    assert txreceipt.status == 1, "should success"

    def check():
        v = erc20.caller.balanceOf(ADDRS["validator"])
        return v == balance

    wait_for_fn("send-to-ethereum", check)


def test_gov_token_mapping(gravity):
    """
    Test adding a token mapping through gov module
    - deploy test erc20 contract on geth
    - deploy corresponding contract on cronos
    - add the token mapping on cronos using gov module
    - do a gravity transfer, check the balances
    """
    geth = gravity.geth
    cli = gravity.cronos.cosmos_cli()
    cronos_w3 = gravity.cronos.w3

    # deploy test erc20 contract
    erc20 = deploy_contract(
        geth,
        Path(__file__).parent
        / "contracts/artifacts/contracts/TestERC20A.sol/TestERC20A.json",
    )
    print("erc20 contract", erc20.address)
    crc20 = deploy_contract(
        cronos_w3,
        Path(__file__).parent
        / "contracts/artifacts/contracts/TestERC20Utility.sol/TestERC20Utility.json",
    )
    print("crc20 contract", crc20.address)
    denom = f"gravity{erc20.address}"

    print("check the contract mapping not exists yet")
    with pytest.raises(AssertionError):
        cli.query_contract_by_denom(denom)

    rsp = cli.gov_propose_token_mapping_change(
        denom, crc20.address, from_="community", deposit="1basetcro"
    )
    assert rsp["code"] == 0, rsp["raw_log"]

    # get proposal_id
    ev = parse_events(rsp["logs"])["submit_proposal"]
    assert ev["proposal_type"] == "TokenMappingChange", rsp
    proposal_id = ev["proposal_id"]
    print("gov proposal submitted", proposal_id)

    # not sure why, but sometimes can't find the proposal immediatelly
    wait_for_new_blocks(cli, 1)
    proposal = cli.query_proposal(proposal_id)

    # each validator vote yes
    for i in range(len(gravity.cronos.config["validators"])):
        rsp = gravity.cronos.cosmos_cli(i).gov_vote("validator", proposal_id, "yes")
        assert rsp["code"] == 0, rsp["raw_log"]
    wait_for_new_blocks(cli, 1)
    assert (
        int(cli.query_tally(proposal_id)["yes"]) == cli.staking_pool()
    ), "all validators should have voted yes"
    print("wait for proposal to be activated")
    wait_for_block_time(cli, isoparse(proposal["voting_end_time"]))
    wait_for_new_blocks(cli, 1)

    print("check the contract mapping exists now")
    rsp = cli.query_contract_by_denom(denom)
    print("contract", rsp)
    assert rsp["contract"] == crc20.address

    print("try to send token from ethereum to cronos")
    txreceipt = send_to_cosmos(
        gravity.contract, erc20, ADDRS["community"], 10, KEYS["validator"]
    )
    assert txreceipt.status == 1

    def check():
        balance = crc20.caller.balanceOf(ADDRS["community"])
        print("crc20 balance", balance)
        return balance == 10

    wait_for_fn("check balance on cronos", check)


def test_direct_token_mapping(gravity):
    """
    Test adding a token mapping directly
    - deploy test erc20 contract on geth
    - deploy corresponding contract on cronos
    - add the token mapping on cronos using gov module
    - do a gravity transfer, check the balances
    """
    geth = gravity.geth
    cli = gravity.cronos.cosmos_cli()
    cronos_w3 = gravity.cronos.w3

    # deploy test erc20 contract
    erc20 = deploy_contract(
        geth,
        Path(__file__).parent
        / "contracts/artifacts/contracts/TestERC20A.sol/TestERC20A.json",
    )
    print("erc20 contract", erc20.address)
    crc20 = deploy_contract(
        cronos_w3,
        Path(__file__).parent
        / "contracts/artifacts/contracts/TestERC20Utility.sol/TestERC20Utility.json",
    )
    print("crc20 contract", crc20.address)
    denom = f"gravity{erc20.address}"

    print("check the contract mapping not exists yet")
    with pytest.raises(AssertionError):
        cli.query_contract_by_denom(denom)

    rsp = cli.update_token_mapping(denom, crc20.address, from_="community")
    assert rsp["code"] != 0, "should not have the permission"

    rsp = cli.update_token_mapping(denom, crc20.address, from_="validator")
    assert rsp["code"] == 0, rsp["raw_log"]
    wait_for_new_blocks(cli, 1)

    print("check the contract mapping exists now")
    rsp = cli.query_contract_by_denom(denom)
    print("contract", rsp)
    assert rsp["contract"] == crc20.address

    print("try to send token from ethereum to cronos")
    txreceipt = send_to_cosmos(
        gravity.contract, erc20, ADDRS["community"], 10, KEYS["validator"]
    )
    assert txreceipt.status == 1

    def check():
        balance = crc20.caller.balanceOf(ADDRS["community"])
        print("crc20 balance", balance)
        return balance == 10

    wait_for_fn("check balance on cronos", check)
