import json

import pytest
import toml
from dateutil.parser import isoparse
from eth_account.account import Account
from eth_utils import abi
from hexbytes import HexBytes
from pystarport import ports

from .gorc import GoRc
from .network import GravityBridge, setup_cronos_experimental, setup_geth
from .utils import (
    ADDRS,
    CONTRACTS,
    KEYS,
    add_ini_sections,
    deploy_contract,
    dump_toml,
    eth_to_bech32,
    parse_events,
    send_to_cosmos,
    send_transaction,
    supervisorctl,
    wait_for_block_time,
    wait_for_fn,
    wait_for_new_blocks,
)

pytestmark = pytest.mark.gravity

Account.enable_unaudited_hdwallet_features()


def cronos_crc21_abi():
    path = CONTRACTS["ModuleCRC21"]
    return json.load(path.open())["abi"]


def gorc_config(keystore, gravity_contract, eth_rpc, cosmos_grpc, metrics_listen):
    return {
        "keystore": str(keystore),
        "gravity": {
            "contract": gravity_contract,
            "fees_denom": "basetcro",
        },
        "ethereum": {
            "key_derivation_path": "m/44'/60'/0'/0/0",
            "rpc": eth_rpc,
        },
        "cosmos": {
            "gas_price": {
                "amount": 5000000000000,
                "denom": "basetcro",
            },
            "grpc": cosmos_grpc,
            "key_derivation_path": "m/44'/60'/0'/0/0",
            "prefix": "crc",
        },
        "metrics": {
            "listen_addr": metrics_listen,
        },
    }


def update_gravity_contract(tomlfile, contract):
    with open(tomlfile) as fp:
        obj = toml.load(fp)
    obj["gravity"]["contract"] = contract
    tomlfile.write_text(dump_toml(obj))


@pytest.fixture(scope="module")
def geth(tmp_path_factory):
    "start-geth"
    for network in setup_geth(tmp_path_factory.mktemp("geth"), 8555):
        yield network.w3


@pytest.fixture(scope="module", params=[True, False])
def cronos(request, tmp_path_factory):
    """start-cronos
    params: enable_auto_deployment
    """
    yield from setup_cronos_experimental(
        tmp_path_factory.mktemp("cronos_experimental"), 26700, request.param
    )


@pytest.fixture(scope="module")
def gravity(cronos, geth):
    """
    - set-delegator-keys
    - deploy gravity contract
    - start orchestrator
    """
    chain_id = "cronos_777-1"

    # set-delegate-keys
    for i, val in enumerate(cronos.config["validators"]):
        # generate gorc config file
        gorc_config_path = cronos.base_dir / f"node{i}/gorc.toml"
        grpc_port = ports.grpc_port(val["base_port"])
        metrics_port = 3000 + i
        gorc_config_path.write_text(
            dump_toml(
                gorc_config(
                    cronos.base_dir / f"node{i}/orchestrator_keystore",
                    "",  # to be filled later after the gravity contract deployed
                    geth.provider.endpoint_uri,
                    f"http://localhost:{grpc_port}",
                    f"127.0.0.1:{metrics_port}",
                )
            )
        )

        gorc = GoRc(gorc_config_path)

        # generate new accounts on both chain
        gorc.add_eth_key("eth")
        gorc.add_eth_key("cronos")  # cronos and eth key derivation are the same

        # fund the orchestrator accounts
        eth_addr = gorc.show_eth_addr("eth")
        print("fund 0.1 eth to address", eth_addr)
        send_transaction(geth, {"to": eth_addr, "value": 10**17}, KEYS["validator"])
        acc_addr = gorc.show_cosmos_addr("cronos")
        print("fund 100cro to address", acc_addr)
        rsp = cronos.cosmos_cli().transfer(
            "community", acc_addr, "%dbasetcro" % (100 * (10**18))
        )
        assert rsp["code"] == 0, rsp["raw_log"]

        cli = cronos.cosmos_cli(i)
        val_addr = cli.address("validator", bech="val")
        val_acct_addr = cli.address("validator")
        nonce = int(cli.account(val_acct_addr)["base_account"]["sequence"])
        signature = gorc.sign_validator("eth", val_addr, nonce)
        rsp = cli.set_delegate_keys(
            val_addr, acc_addr, eth_addr, HexBytes(signature).hex(), from_=val_acct_addr
        )
        assert rsp["code"] == 0, rsp["raw_log"]
    # wait for gravity signer tx get generated
    wait_for_new_blocks(cli, 2)

    # deploy gravity contract to geth
    gravity_id = cli.query_gravity_params()["params"]["gravity_id"]
    signer_set = cli.query_latest_signer_set_tx()["signer_set"]["signers"]
    powers = [int(signer["power"]) for signer in signer_set]
    threshold = int(2**32 * 0.66)  # gravity normalize the power to [0, 2**32]
    eth_addresses = [signer["ethereum_address"] for signer in signer_set]
    assert sum(powers) >= threshold, "not enough validator on board"

    contract = deploy_contract(
        geth,
        CONTRACTS["Gravity"],
        (gravity_id.encode(), threshold, eth_addresses, powers),
    )
    print("gravity contract deployed", contract.address)

    # start orchestrator:
    # a) add process into the supervisord config file
    # b) reload supervisord
    programs = {}
    for i, val in enumerate(cronos.config["validators"]):
        # update gravity contract in gorc config
        gorc_config_path = cronos.base_dir / f"node{i}/gorc.toml"
        update_gravity_contract(gorc_config_path, contract.address)

        programs[f"program:{chain_id}-orchestrator{i}"] = {
            "command": (
                f'gorc -c "{gorc_config_path}" orchestrator start '
                "--cosmos-key cronos --ethereum-key eth"
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


def test_gravity_transfer(gravity):
    geth = gravity.geth
    cli = gravity.cronos.cosmos_cli()
    cronos_w3 = gravity.cronos.w3

    # deploy test erc20 contract
    erc20 = deploy_contract(
        geth,
        CONTRACTS["TestERC20A"],
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

    denom = f"gravity{erc20.address}"

    crc21_contract = None

    def check_auto_deployment():
        "check crc20 contract auto deployed, and the crc20 balance"
        nonlocal crc21_contract
        try:
            rsp = cli.query_contract_by_denom(denom)
        except AssertionError:
            # not deployed yet
            return False
        assert len(rsp["auto_contract"]) > 0
        crc21_contract = cronos_w3.eth.contract(
            address=rsp["auto_contract"], abi=cronos_crc21_abi()
        )
        return crc21_contract.caller.balanceOf(recipient) == amount

    def check_gravity_native_tokens():
        "check the balance of gravity native token"
        return cli.balance(eth_to_bech32(recipient), denom=denom) == amount

    def get_id_from_receipt(receipt):
        "check the id after sendToChain call"
        for _, log in enumerate(receipt.logs):
            if log.topics[0] == HexBytes(
                abi.event_signature_to_log_topic("__CronosSendToChainResponse(uint256)")
            ):
                return log.data
        return "0x0000000000000000000000000000000000000000000000000000000000000000"

    if gravity.cronos.enable_auto_deployment:
        wait_for_fn("send-to-crc20", check_auto_deployment)

        # send it back to erc20
        tx = crc21_contract.functions.send_to_chain(
            ADDRS["validator"], amount, 0, 1
        ).buildTransaction({"from": ADDRS["community"]})
        txreceipt = send_transaction(cronos_w3, tx, KEYS["community"])
        # CRC20 emit 3 logs for send_to_chain:
        # burn
        # __CronosSendToChain
        # __CronosSendToChainResponse
        assert len(txreceipt.logs) == 3
        assert (
            get_id_from_receipt(txreceipt)
            == "0x0000000000000000000000000000000000000000000000000000000000000001"
        ), "should be able to get id"
        assert txreceipt.status == 1, "should success"
    else:
        wait_for_fn("send-to-gravity-native", check_gravity_native_tokens)
        # send back the gravity native tokens
        rsp = cli.send_to_ethereum(
            ADDRS["validator"], f"{amount}{denom}", f"0{denom}", from_="community"
        )
        assert rsp["code"] == 0, rsp["raw_log"]

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
        CONTRACTS["TestERC20A"],
    )
    print("erc20 contract", erc20.address)
    crc21 = deploy_contract(
        cronos_w3,
        CONTRACTS["TestERC20Utility"],
    )
    print("crc21 contract", crc21.address)
    denom = f"gravity{erc20.address}"

    print("check the contract mapping not exists yet")
    with pytest.raises(AssertionError):
        cli.query_contract_by_denom(denom)

    rsp = cli.gov_propose_token_mapping_change(
        denom, crc21.address, from_="community", deposit="1basetcro"
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
    assert rsp["contract"] == crc21.address

    print("try to send token from ethereum to cronos")
    txreceipt = send_to_cosmos(
        gravity.contract, erc20, ADDRS["community"], 10, KEYS["validator"]
    )
    assert txreceipt.status == 1

    def check():
        balance = crc21.caller.balanceOf(ADDRS["community"])
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
        CONTRACTS["TestERC20A"],
    )
    print("erc20 contract", erc20.address)
    crc21 = deploy_contract(
        cronos_w3,
        CONTRACTS["TestERC20Utility"],
    )
    print("crc21 contract", crc21.address)
    denom = f"gravity{erc20.address}"

    print("check the contract mapping not exists yet")
    with pytest.raises(AssertionError):
        cli.query_contract_by_denom(denom)

    rsp = cli.update_token_mapping(denom, crc21.address, from_="community")
    assert rsp["code"] != 0, "should not have the permission"

    rsp = cli.update_token_mapping(denom, crc21.address, from_="validator")
    assert rsp["code"] == 0, rsp["raw_log"]
    wait_for_new_blocks(cli, 1)

    print("check the contract mapping exists now")
    rsp = cli.query_contract_by_denom(denom)
    print("contract", rsp)
    assert rsp["contract"] == crc21.address

    print("try to send token from ethereum to cronos")
    txreceipt = send_to_cosmos(
        gravity.contract, erc20, ADDRS["community"], 10, KEYS["validator"]
    )
    assert txreceipt.status == 1

    def check():
        balance = crc21.caller.balanceOf(ADDRS["community"])
        print("crc20 balance", balance)
        return balance == 10

    wait_for_fn("check balance on cronos", check)


def test_gravity_cancel_transfer(gravity):
    if gravity.cronos.enable_auto_deployment:
        geth = gravity.geth
        cli = gravity.cronos.cosmos_cli()
        cronos_w3 = gravity.cronos.w3

        # deploy test erc20 contract
        erc20 = deploy_contract(
            geth,
            CONTRACTS["TestERC20A"],
        )

        # deploy gravity cancellation contract
        cancel_contract = deploy_contract(
            cronos_w3,
            CONTRACTS["CronosGravityCancellation"],
        )

        balance = erc20.caller.balanceOf(ADDRS["validator"])
        assert balance == 100000000000000000000000000
        amount = 1000

        print("send to cronos crc21")
        recipient = HexBytes(ADDRS["community"])
        send_to_cosmos(gravity.contract, erc20, recipient, amount, KEYS["validator"])
        assert erc20.caller.balanceOf(ADDRS["validator"]) == balance - amount

        denom = f"gravity{erc20.address}"

        crc21_contract = None

        def check_auto_deployment():
            "check crc21 contract auto deployed, and the crc21 balance"
            nonlocal crc21_contract
            try:
                rsp = cli.query_contract_by_denom(denom)
            except AssertionError:
                # not deployed yet
                return False
            assert len(rsp["auto_contract"]) > 0
            crc21_contract = cronos_w3.eth.contract(
                address=rsp["auto_contract"], abi=cronos_crc21_abi()
            )
            return crc21_contract.caller.balanceOf(recipient) == amount

        def get_id_from_receipt(receipt):
            "check the id after sendToChain call"
            for _, log in enumerate(receipt.logs):
                if log.topics[0] == HexBytes(
                    abi.event_signature_to_log_topic(
                        "__CronosSendToChainResponse(uint256)"
                    )
                ):
                    return log.data
            return "0x0000000000000000000000000000000000000000000000000000000000000000"

        wait_for_fn("send-to-crc20", check_auto_deployment)

        def check_fund():
            v = crc21_contract.caller.balanceOf(ADDRS["community"])
            return v == amount

        wait_for_fn("send-to-ethereum", check_fund)

        # send it back to erc20
        tx = crc21_contract.functions.send_to_chain(
            ADDRS["validator"], amount, 0, 1
        ).buildTransaction({"from": ADDRS["community"]})
        txreceipt = send_transaction(cronos_w3, tx, KEYS["community"])
        # CRC20 emit 3 logs for send_to_chain:
        # burn
        # __CronosSendToChain
        # __CronosSendToChainResponse
        assert len(txreceipt.logs) == 3
        tx_id = get_id_from_receipt(txreceipt)
        assert txreceipt.status == 1, "should success"

        # Check_deduction
        balance_after_send = crc21_contract.caller.balanceOf(ADDRS["community"])
        assert balance_after_send == 0

        # Cancel the send_to_chain
        canceltx = cancel_contract.functions.cancelTransaction(
            int(tx_id, base=16)
        ).buildTransaction({"from": ADDRS["community"]})
        canceltxreceipt = send_transaction(cronos_w3, canceltx, KEYS["community"])
        print("canceltxreceipt", canceltxreceipt)
        assert canceltxreceipt.status == 1, "should success"

        def check_refund():
            v = crc21_contract.caller.balanceOf(ADDRS["community"])
            return v == amount

        wait_for_fn("cancel-send-to-ethereum", check_refund)
